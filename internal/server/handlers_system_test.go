package server

import "testing"

func TestParseNVIDIASMIOutput(t *testing.T) {
	out := []byte("NVIDIA GeForce RTX 4090, 1536, 24564\n")

	info, ok := parseNVIDIASMIOutput(out)
	if !ok {
		t.Fatal("parseNVIDIASMIOutput returned false, want true")
	}
	if info.Name != "NVIDIA GeForce RTX 4090" {
		t.Fatalf("unexpected GPU name: %q", info.Name)
	}
	if info.VRAMUsed != 1536*1024*1024 {
		t.Fatalf("unexpected GPU used bytes: %d", info.VRAMUsed)
	}
	if info.VRAMTotal != 24564*1024*1024 {
		t.Fatalf("unexpected GPU total bytes: %d", info.VRAMTotal)
	}
	if !info.UsageAvailable {
		t.Fatal("UsageAvailable = false, want true")
	}
	if info.SharedMemory {
		t.Fatal("SharedMemory = true, want false")
	}
}

func TestParseNVIDIASMIOutputGB10UnifiedMemory(t *testing.T) {
	out := []byte("NVIDIA GB10, Not Supported, Not Supported\n")

	info, ok := parseNVIDIASMIOutput(out)
	if !ok {
		t.Fatal("parseNVIDIASMIOutput returned false, want true")
	}
	if info.Name != "NVIDIA GB10" {
		t.Fatalf("unexpected GPU name: %q", info.Name)
	}
	if info.VRAMUsed != 0 || info.VRAMTotal != 0 {
		t.Fatalf("unexpected GPU memory values: used=%d total=%d", info.VRAMUsed, info.VRAMTotal)
	}
	if info.UsageAvailable {
		t.Fatal("UsageAvailable = true, want false")
	}
	if !info.SharedMemory {
		t.Fatal("SharedMemory = false, want true")
	}
}

func TestParseDarwinSystemProfilerOutputAppleSilicon(t *testing.T) {
	out := []byte(`{
  "SPDisplaysDataType" : [
    {
      "_name" : "Apple M2 Pro",
      "spdisplays_vendor" : "sppci_vendor_Apple",
      "sppci_device_type" : "spdisplays_gpu",
      "sppci_model" : "Apple M2 Pro"
    }
  ]
}`)

	name, total, shared := parseDarwinSystemProfilerOutput(out)
	if name != "Apple M2 Pro" {
		t.Fatalf("unexpected GPU name: %q", name)
	}
	if total != 0 {
		t.Fatalf("unexpected GPU total bytes: %d", total)
	}
	if !shared {
		t.Fatal("shared = false, want true")
	}
}

func TestParseDarwinSystemProfilerOutputDiscreteVRAM(t *testing.T) {
	out := []byte(`{
  "SPDisplaysDataType" : [
    {
      "_name" : "Radeon Pro 5500M",
      "spdisplays_vendor" : "sppci_vendor_AMD",
      "spdisplays_vram" : "4 GB",
      "sppci_model" : "Radeon Pro 5500M"
    }
  ]
}`)

	name, total, shared := parseDarwinSystemProfilerOutput(out)
	if name != "Radeon Pro 5500M" {
		t.Fatalf("unexpected GPU name: %q", name)
	}
	if total != 4*1024*1024*1024 {
		t.Fatalf("unexpected GPU total bytes: %d", total)
	}
	if shared {
		t.Fatal("shared = true, want false")
	}
}

func TestParseMemoryString(t *testing.T) {
	if got := parseMemoryString("Dynamic, Max: 48 GB"); got != 48*1024*1024*1024 {
		t.Fatalf("parseMemoryString returned %d, want %d", got, 48*1024*1024*1024)
	}
}

func TestMemoryFieldUnsupported(t *testing.T) {
	cases := []string{"N/A", "Not Supported", "not supported"}
	for _, input := range cases {
		if !memoryFieldUnsupported(input) {
			t.Fatalf("memoryFieldUnsupported(%q) = false, want true", input)
		}
	}
}
