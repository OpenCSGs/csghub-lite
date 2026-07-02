package apps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const codexAppBundleName = "Codex.app"

var xmlPlistStringPattern = regexp.MustCompile(`<key>([^<]+)</key>\s*<string>([^<]*)</string>`)

// CodexAppLaunchTarget resolves the local Codex App bundle path for desktop launch.
func CodexAppLaunchTarget() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("Codex App is installed, but the user home directory was not found")
	}

	if target, ok := readCodexAppLaunchTargetFile(home); ok {
		return target, nil
	}
	if bundle, ok := findDarwinCodexAppBundle(home); ok {
		return bundle, nil
	}
	if binary, _, ok := findWindowsCodexAppBinary(home); ok {
		return binary, nil
	}

	return "", fmt.Errorf("Codex App is installed, but no launch target was found")
}

func codexAppRuntimeRoot(home string) string {
	return filepath.Join(home, ".local", "share", "codex-app")
}

func codexAppLauncherCandidates(home string) []string {
	dir := filepath.Join(home, ".local", "bin")
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(dir, "codex-app.cmd"),
			filepath.Join(dir, "codex-app.exe"),
		}
	}
	return []string{filepath.Join(dir, "codex-app")}
}

func codexAppLauncherPath(home string) string {
	candidates := codexAppLauncherCandidates(home)
	return candidates[0]
}

func existingCodexAppLauncher(home string) (string, bool) {
	for _, candidate := range codexAppLauncherCandidates(home) {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func readCodexAppVersionFile(home, fallback string) string {
	versionPath := filepath.Join(codexAppRuntimeRoot(home), "version")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return fallback
	}
	if trimmed := strings.TrimSpace(string(data)); trimmed != "" {
		return trimmed
	}
	return fallback
}

func detectCodexAppInstall() (string, string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", "", false
	}
	if launcherPath, ok := existingCodexAppLauncher(home); ok {
		return launcherPath, readCodexAppVersionFile(home, launcherPath), true
	}
	if target, ok := readCodexAppLaunchTargetFile(home); ok {
		return target, readCodexAppVersionFile(home, readCodexAppTargetVersion(target)), true
	}
	if bundle, ok := findDarwinCodexAppBundle(home); ok {
		return bundle, readDarwinBundleShortVersion(bundle), true
	}
	if binary, version, ok := findWindowsCodexAppBinary(home); ok {
		return binary, version, true
	}
	return "", "", false
}

func readCodexAppTargetVersion(target string) string {
	if runtime.GOOS == "darwin" && strings.HasSuffix(target, ".app") {
		return readDarwinBundleShortVersion(target)
	}
	return target
}

func findWindowsCodexAppBinary(home string) (string, string, bool) {
	if runtime.GOOS != "windows" {
		return "", "", false
	}
	runtimeRoot := filepath.Join(codexAppRuntimeRoot(home), "versions")
	entries, err := os.ReadDir(runtimeRoot)
	if err != nil {
		return "", "", false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		versionDir := filepath.Join(runtimeRoot, entry.Name())
		dirEntries, err := os.ReadDir(versionDir)
		if err != nil {
			continue
		}
		for _, file := range dirEntries {
			if file.IsDir() {
				continue
			}
			name := strings.ToLower(file.Name())
			if !strings.HasSuffix(name, ".exe") {
				continue
			}
			candidate := filepath.Join(versionDir, file.Name())
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, readCodexAppVersionFile(home, entry.Name()), true
			}
		}
	}
	return "", "", false
}

func looksLikeCodexAppInstall(installPath string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || installPath == "" {
		return false
	}
	if launcherPath, ok := existingCodexAppLauncher(home); ok && samePath(installPath, launcherPath) {
		target, ok := readCodexAppLaunchTargetFile(home)
		return ok && target != ""
	}
	return false
}

func readCodexAppLaunchTargetFile(home string) (string, bool) {
	targetPath := filepath.Join(codexAppRuntimeRoot(home), "launch-target")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return "", false
	}
	target := strings.TrimSpace(string(data))
	if target == "" {
		return "", false
	}
	if _, err := os.Stat(target); err != nil {
		return "", false
	}
	return target, true
}

func darwinCodexAppBundleCandidates(home string) []string {
	candidates := make([]string, 0, 2)
	if home != "" {
		candidates = append(candidates, filepath.Join(home, "Applications", codexAppBundleName))
	}
	candidates = append(candidates, filepath.Join("/Applications", codexAppBundleName))
	return candidates
}

func findDarwinCodexAppBundle(home string) (string, bool) {
	if runtime.GOOS != "darwin" {
		return "", false
	}
	return findExistingDirectory(darwinCodexAppBundleCandidates(home))
}

func findExistingDirectory(paths []string) (string, bool) {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func readDarwinBundleShortVersion(bundlePath string) string {
	plistPath := filepath.Join(bundlePath, "Contents", "Info.plist")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return bundlePath
	}
	if version := parseXMLPlistString(data, "CFBundleShortVersionString"); version != "" {
		return version
	}
	if runtime.GOOS == "darwin" {
		if version := readDefaultsBundleVersion(bundlePath); version != "" {
			return version
		}
	}
	return bundlePath
}

func parseXMLPlistString(data []byte, key string) string {
	matches := xmlPlistStringPattern.FindAllSubmatch(data, -1)
	for _, match := range matches {
		if string(match[1]) != key {
			continue
		}
		version := strings.TrimSpace(string(match[2]))
		if version != "" {
			return version
		}
	}
	return ""
}

func readDefaultsBundleVersion(bundlePath string) string {
	out, err := exec.Command("defaults", "read", bundlePath, "CFBundleShortVersionString").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
