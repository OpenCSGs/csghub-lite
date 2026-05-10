package codexagent

import (
	"strings"
	"testing"
)

func TestParseTomlFilePreservesMCPArgsArray(t *testing.T) {
	values := map[string]tomlValue{}
	parseTomlFile(`
[mcp_servers.remotion-documentation]
command = "npx"
args = ["@remotion/mcp@latest"]
`, values)

	args, ok := values["mcp_servers.remotion-documentation.args"]
	if !ok {
		t.Fatal("missing mcp args")
	}
	if !args.isRaw {
		t.Fatalf("mcp args parsed as quoted string: %#v", args)
	}
	if got := strings.TrimSpace(formatTomlKV("args", args)); got != `args = ["@remotion/mcp@latest"]` {
		t.Fatalf("formatted args = %q", got)
	}
}

func TestParseTomlFileRepairsLegacyEncodedMCPArgsArray(t *testing.T) {
	values := map[string]tomlValue{}
	parseTomlFile(`mcp_servers.remotion-documentation.args = "[\\\"@remotion/mcp@latest\\\"]"`, values)

	args, ok := values["mcp_servers.remotion-documentation.args"]
	if !ok {
		t.Fatal("missing mcp args")
	}
	if !args.isRaw {
		t.Fatalf("legacy mcp args parsed as quoted string: %#v", args)
	}
	if got := strings.TrimSpace(formatTomlKV("args", args)); got != `args = ["@remotion/mcp@latest"]` {
		t.Fatalf("formatted repaired args = %q", got)
	}
}

func TestParseTomlFilePreservesMultilineMCPArgsArray(t *testing.T) {
	values := map[string]tomlValue{}
	parseTomlFile(`
[mcp_servers.example]
args = [
  "first",
  "second",
]
`, values)

	args, ok := values["mcp_servers.example.args"]
	if !ok {
		t.Fatal("missing mcp args")
	}
	if !args.isRaw {
		t.Fatalf("multiline args parsed as quoted string: %#v", args)
	}
	formatted := formatTomlKV("args", args)
	for _, want := range []string{`args = [`, `"first"`, `"second"`, `]`} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted multiline args missing %q:\n%s", want, formatted)
		}
	}
}

func TestParseTomlFilePreservesRawScalarAndInlineTableValues(t *testing.T) {
	values := map[string]tomlValue{}
	parseTomlFile(`
retry_delay = 1.5
release_date = 2026-05-10T11:00:00Z
headers = { Authorization = "Bearer token", Accept = "application/json" }
quoted = "value"
`, values)

	for _, key := range []string{"retry_delay", "release_date", "headers"} {
		value := values[key]
		if !value.isRaw {
			t.Fatalf("%s parsed as non-raw value: %#v", key, value)
		}
	}
	if values["quoted"].isRaw || values["quoted"].strVal != "value" {
		t.Fatalf("quoted string parsed incorrectly: %#v", values["quoted"])
	}
}
