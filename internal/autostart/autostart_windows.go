package autostart

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	registryKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	registryValue = "CSGHubLite"
)

func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// IsEnabled reports whether the Windows Run registry entry exists.
func IsEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.QUERY_VALUE)
	if err != nil {
		return false, nil
	}
	defer key.Close()

	_, _, err = key.GetStringValue(registryValue)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Enable creates a Windows Run registry entry so csghub-lite starts on login.
func Enable() error {
	binary, err := executablePath()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("opening registry key: %w", err)
	}
	defer key.Close()

	cmd := fmt.Sprintf(`"%s" serve`, binary)
	return key.SetStringValue(registryValue, cmd)
}

// Disable removes the Windows Run registry entry.
func Disable() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		return nil // Key doesn't exist, nothing to disable.
	}
	defer key.Close()

	if err := key.DeleteValue(registryValue); err != nil {
		// Value already missing is fine.
		return nil
	}
	return nil
}
