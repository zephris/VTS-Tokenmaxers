package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	contents := "VTS_TEST_PLAIN=value # comment\nVTS_TEST_QUOTED=\"hello world\"\nVTS_TEST_JSON=[\"one\",\"two\"]\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"VTS_TEST_PLAIN", "VTS_TEST_QUOTED", "VTS_TEST_JSON"} {
		t.Cleanup(func() { _ = os.Unsetenv(name) })
		_ = os.Unsetenv(name)
	}
	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("VTS_TEST_PLAIN"); got != "value" {
		t.Fatalf("plain value = %q", got)
	}
	if got := os.Getenv("VTS_TEST_QUOTED"); got != "hello world" {
		t.Fatalf("quoted value = %q", got)
	}
	if got := os.Getenv("VTS_TEST_JSON"); got != `["one","two"]` {
		t.Fatalf("JSON value = %q", got)
	}
}

func TestLoadDotEnvDoesNotOverrideEnvironment(t *testing.T) {
	t.Setenv("VTS_TEST_EXISTING", "from-environment")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("VTS_TEST_EXISTING=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("VTS_TEST_EXISTING"); got != "from-environment" {
		t.Fatalf("existing value was overwritten with %q", got)
	}
}
