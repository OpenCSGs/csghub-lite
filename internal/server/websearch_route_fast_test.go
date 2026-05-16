package server

import (
	"strings"
	"testing"
	"time"
)

func TestTryFastWebSearchRouteSkipGreeting(t *testing.T) {
	route, ok := tryFastWebSearchRoute("你好", time.Now())
	if !ok || route.Action != webSearchActionSkip {
		t.Fatalf("route = %#v, ok = %v", route, ok)
	}
}

func TestTryFastWebSearchRouteSearchNews(t *testing.T) {
	now := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	route, ok := tryFastWebSearchRoute("latest news", now)
	if !ok || route.Action != webSearchActionSearch || route.Query == "" {
		t.Fatalf("route = %#v, ok = %v", route, ok)
	}
}

func TestTryFastWebSearchRouteSearchExplicitLookup(t *testing.T) {
	route, ok := tryFastWebSearchRoute("查一下 Qwen 最新版本", time.Now())
	if !ok || route.Action != webSearchActionSearch || !strings.Contains(route.Query, "Qwen") {
		t.Fatalf("route = %#v, ok = %v", route, ok)
	}
}

func TestTryFastWebSearchRouteAmbiguousNeedsLLM(t *testing.T) {
	_, ok := tryFastWebSearchRoute("解释一下 Go slice", time.Now())
	if ok {
		t.Fatal("expected ambiguous query to use LLM routing")
	}
}
