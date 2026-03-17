package server

import (
	"fmt"
	"math"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// float2 is a float64 that marshals to JSON with at most 2 decimal places.
type float2 float64

func (f float2) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.2f", float64(f))), nil
}

type systemInfo struct {
	CPUCores     int    `json:"cpu_cores"`
	CPUUsage     float2 `json:"cpu_usage"`
	CPUClock     string `json:"cpu_clock"`
	RAMUsed      uint64 `json:"ram_used"`
	RAMTotal     uint64 `json:"ram_total"`
	RAMInfo      string `json:"ram_info"`
	GPUName      string `json:"gpu_name"`
	GPUVRAMUsed  uint64 `json:"gpu_vram_used"`
	GPUVRAMTotal uint64 `json:"gpu_vram_total"`
}

// GET /api/system -- system resource information
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	info := systemInfo{
		CPUCores: runtime.NumCPU(),
	}

	cpuUsage, cpuClock := getCPUInfo()
	info.CPUUsage = float2(cpuUsage)
	info.CPUClock = cpuClock
	info.RAMUsed, info.RAMTotal, info.RAMInfo = getRAMInfo()
	info.GPUName, info.GPUVRAMUsed, info.GPUVRAMTotal = getGPUInfo()

	writeJSON(w, http.StatusOK, info)
}

func getCPUInfo() (usage float64, clock string) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
		if err == nil {
			clock = strings.TrimSpace(string(out))
		}
		// CPU usage via top
		out, err = exec.Command("sh", "-c", "top -l 1 -n 0 | grep 'CPU usage'").Output()
		if err == nil {
			line := string(out)
			if idx := strings.Index(line, "idle"); idx > 0 {
				parts := strings.Fields(line[:idx])
				if len(parts) > 0 {
					idle := strings.TrimSuffix(parts[len(parts)-1], "%")
					if v, e := strconv.ParseFloat(idle, 64); e == nil {
						usage = math.Round((100-v)*100) / 100
					}
				}
			}
		}
	case "linux":
		out, err := exec.Command("sh", "-c", "lscpu | grep 'Model name'").Output()
		if err == nil {
			parts := strings.SplitN(string(out), ":", 2)
			if len(parts) == 2 {
				clock = strings.TrimSpace(parts[1])
			}
		}
		out, err = exec.Command("sh", "-c", "grep 'cpu ' /proc/stat").Output()
		if err == nil {
			fields := strings.Fields(string(out))
			if len(fields) >= 5 {
				user, _ := strconv.ParseFloat(fields[1], 64)
				system, _ := strconv.ParseFloat(fields[3], 64)
				idle, _ := strconv.ParseFloat(fields[4], 64)
				total := user + system + idle
				if total > 0 {
					usage = math.Round((user+system)/total*10000) / 100
				}
			}
		}
	}
	return
}

func getRAMInfo() (used, total uint64, info string) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			total, _ = strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
		}
		out, err = exec.Command("sh", "-c", "vm_stat | grep 'Pages active\\|Pages wired'").Output()
		if err == nil {
			var pages uint64
			for _, line := range strings.Split(string(out), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					val := strings.TrimSuffix(fields[len(fields)-1], ".")
					if v, e := strconv.ParseUint(val, 10, 64); e == nil {
						pages += v
					}
				}
			}
			used = pages * 16384 // macOS page size is 16KB
		}
	case "linux":
		out, err := exec.Command("sh", "-c", "grep -E 'MemTotal|MemAvailable' /proc/meminfo").Output()
		if err == nil {
			var memTotal, memAvail uint64
			for _, line := range strings.Split(string(out), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					val, _ := strconv.ParseUint(fields[1], 10, 64)
					val *= 1024 // kB to bytes
					if strings.HasPrefix(line, "MemTotal") {
						memTotal = val
					} else if strings.HasPrefix(line, "MemAvailable") {
						memAvail = val
					}
				}
			}
			total = memTotal
			used = memTotal - memAvail
		}
	}
	return
}

func getGPUInfo() (name string, used, total uint64) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,memory.used,memory.total",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		switch runtime.GOOS {
		case "darwin":
			out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
			if err == nil {
				for _, line := range strings.Split(string(out), "\n") {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "Chipset Model:") || strings.HasPrefix(trimmed, "Chip:") {
						parts := strings.SplitN(trimmed, ":", 2)
						if len(parts) == 2 {
							name = strings.TrimSpace(parts[1])
						}
					}
					if strings.Contains(trimmed, "VRAM") || strings.Contains(trimmed, "Memory") {
						parts := strings.SplitN(trimmed, ":", 2)
						if len(parts) == 2 {
							val := strings.TrimSpace(parts[1])
							val = strings.TrimSuffix(val, " MB")
							val = strings.TrimSuffix(val, " GB")
							if v, e := strconv.ParseUint(val, 10, 64); e == nil {
								if strings.Contains(parts[1], "GB") {
									total = v * 1024 * 1024 * 1024
								} else {
									total = v * 1024 * 1024
								}
							}
						}
					}
				}
			}
		}
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		fields := strings.Split(lines[0], ",")
		if len(fields) >= 3 {
			name = strings.TrimSpace(fields[0])
			if v, e := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 64); e == nil {
				used = v * 1024 * 1024 // MiB to bytes
			}
			if v, e := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 64); e == nil {
				total = v * 1024 * 1024 // MiB to bytes
			}
		}
	}
	return
}
