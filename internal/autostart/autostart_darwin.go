package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	plistLabel = "com.opencsg.csghub-lite"
	plistName  = plistLabel + ".plist"
)

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistName), nil
}

func executablePath() (string, error) {
	return os.Executable()
}

func plistContent(binary string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>serve</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
</dict>
</plist>
`, plistLabel, binary)
}

// IsEnabled reports whether the LaunchAgent plist exists.
func IsEnabled() (bool, error) {
	path, err := plistPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// Enable creates a LaunchAgent plist so csghub-lite starts on login.
func Enable() error {
	binary, err := executablePath()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	// Resolve symlinks so the plist points to the real binary.
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return fmt.Errorf("resolving executable symlink: %w", err)
	}

	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(plistContent(binary)), 0o644)
}

// Disable removes the LaunchAgent plist.
func Disable() error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// PlistContainsBinary checks whether the current plist references the
// running binary — useful for tests. Exported only for testing.
func PlistContainsBinary() (bool, error) {
	path, err := plistPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	binary, err := executablePath()
	if err != nil {
		return false, err
	}
	binary, _ = filepath.EvalSymlinks(binary)
	return strings.Contains(string(data), binary), nil
}
