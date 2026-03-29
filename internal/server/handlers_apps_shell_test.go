package server

import (
	"testing"
	"time"
)

func waitForShellSession(t *testing.T, timeout time.Duration, check func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

func TestAIAppShellSessionTouchActivityExtendsIdleTimeout(t *testing.T) {
	oldTimeout := aiAppShellIdleTimeout
	aiAppShellIdleTimeout = 120 * time.Millisecond
	t.Cleanup(func() {
		aiAppShellIdleTimeout = oldTimeout
	})

	manager := newAIAppShellManager()
	session := &aiAppShellSession{
		manager: manager,
		id:      "touch-activity",
		subs:    make(map[chan aiAppShellEvent]struct{}),
	}
	manager.sessions[session.id] = session
	session.scheduleIdleTimeout()

	time.Sleep(70 * time.Millisecond)
	session.touchActivity()

	if _, ok := manager.Get(session.id); !ok {
		t.Fatal("session expired before the refreshed timeout elapsed")
	}

	time.Sleep(70 * time.Millisecond)
	if _, ok := manager.Get(session.id); !ok {
		t.Fatal("session expired too early after touchActivity")
	}

	waitForShellSession(t, 250*time.Millisecond, func() bool {
		_, ok := manager.Get(session.id)
		return !ok
	}, "session did not expire after the refreshed idle timeout")
}

func TestAIAppShellSessionAttachStillExpiresWhenIdle(t *testing.T) {
	oldTimeout := aiAppShellIdleTimeout
	aiAppShellIdleTimeout = 100 * time.Millisecond
	t.Cleanup(func() {
		aiAppShellIdleTimeout = oldTimeout
	})

	manager := newAIAppShellManager()
	session := &aiAppShellSession{
		manager: manager,
		id:      "attach-timeout",
		subs:    make(map[chan aiAppShellEvent]struct{}),
	}
	manager.sessions[session.id] = session
	session.scheduleIdleTimeout()

	attach := session.Attach()
	if attach.events == nil {
		t.Fatal("expected live attach events channel")
	}
	defer session.Detach(attach.events)

	waitForShellSession(t, 250*time.Millisecond, func() bool {
		_, ok := manager.Get(session.id)
		return !ok
	}, "attached idle session did not expire after idle timeout")
}
