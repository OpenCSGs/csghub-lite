package apps

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectCodexAppInstallFromUserApplications(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin Applications detection")
	}

	home := setTempHome(t)
	bundle := writeDarwinCodexAppBundle(t, filepath.Join(home, "Applications"), "26.616.31447")

	installPath, version, ok := detectCodexAppInstall()
	if !ok {
		t.Fatal("expected Codex App in ~/Applications to be detected")
	}
	if installPath != bundle {
		t.Fatalf("install path = %q, want %q", installPath, bundle)
	}
	if version != "26.616.31447" {
		t.Fatalf("version = %q, want 26.616.31447", version)
	}

	mgr := NewManager(nil)
	info, err := mgr.Get(t.Context(), "codex-app")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !info.Installed {
		t.Fatal("expected codex-app to be detected as installed")
	}
	if info.Managed {
		t.Fatal("expected Applications install to remain unmanaged")
	}
}

func TestCodexAppLaunchTargetPrefersManagedLaunchTarget(t *testing.T) {
	home := setTempHome(t)

	managedBundle := filepath.Join(home, ".local", "share", "codex-app", "versions", "26.527.31326", codexAppBundleName)
	if runtime.GOOS == "windows" {
		managedBundle = filepath.Join(home, ".local", "share", "codex-app", "versions", "26.527.31326", "Codex.exe")
		if err := os.MkdirAll(filepath.Dir(managedBundle), 0o755); err != nil {
			t.Fatalf("mkdir exe dir: %v", err)
		}
		if err := os.WriteFile(managedBundle, []byte("stub"), 0o644); err != nil {
			t.Fatalf("write exe stub: %v", err)
		}
	} else if err := os.MkdirAll(managedBundle, 0o755); err != nil {
		t.Fatalf("mkdir managed bundle: %v", err)
	}

	userBundle := writeDarwinCodexAppBundle(t, filepath.Join(home, "Applications"), "1.0.0")
	runtimeRoot := codexAppRuntimeRoot(home)
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatalf("mkdir runtime root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runtimeRoot, "launch-target"), []byte(managedBundle+"\n"), 0o644); err != nil {
		t.Fatalf("write launch target: %v", err)
	}

	got, err := CodexAppLaunchTarget()
	if err != nil {
		t.Fatalf("CodexAppLaunchTarget() error: %v", err)
	}
	if got != managedBundle {
		t.Fatalf("CodexAppLaunchTarget() = %q, want %q", got, managedBundle)
	}
	_ = userBundle
}

func TestCodexAppLaunchTargetFallsBackToUserApplications(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin Applications fallback")
	}

	home := setTempHome(t)
	bundle := writeDarwinCodexAppBundle(t, filepath.Join(home, "Applications"), "26.616.31447")

	got, err := CodexAppLaunchTarget()
	if err != nil {
		t.Fatalf("CodexAppLaunchTarget() error: %v", err)
	}
	if got != bundle {
		t.Fatalf("CodexAppLaunchTarget() = %q, want %q", got, bundle)
	}
}

func writeDarwinCodexAppBundle(t *testing.T, parentDir, version string) string {
	t.Helper()

	bundle := filepath.Join(parentDir, codexAppBundleName)
	contentsDir := filepath.Join(bundle, "Contents")
	if err := os.MkdirAll(contentsDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle contents: %v", err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleShortVersionString</key>
  <string>` + version + `</string>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(contentsDir, "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatalf("write Info.plist: %v", err)
	}
	return bundle
}
