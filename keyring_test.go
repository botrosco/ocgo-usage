package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLookupAuthFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	content := `{
	  "opencode-go": { "type": "api_key", "key": "sk-1234567890abcdef" },
	  "opencode":    { "type": "oauth", "access": "tok-abc", "refresh": "r", "expires": 0 },
	  "anthropic":   { "type": "api_key", "key": "sk-ant-zzz" }
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// opencode-go preferred entry -> api key.
	ks, ok := lookupAuthFile(path, "opencode-go")
	if !ok || ks.key != "sk-1234567890abcdef" {
		t.Fatalf("opencode-go lookup: got %+v ok=%v", ks, ok)
	}

	// oauth entry -> access token.
	ks, ok = lookupAuthFile(path, "opencode")
	if !ok || ks.key != "tok-abc" {
		t.Fatalf("opencode lookup: got %+v ok=%v", ks, ok)
	}

	// missing entry -> not found.
	if _, ok := lookupAuthFile(path, "nope"); ok {
		t.Fatal("expected not found for missing entry")
	}

	// missing file -> not found, no error.
	if _, ok := lookupAuthFile(filepath.Join(dir, "nope.json"), "opencode-go"); ok {
		t.Fatal("expected not found for missing file")
	}
}

func TestResolveKeyPrecedence(t *testing.T) {
	dir := t.TempDir()
	piAuth := filepath.Join(dir, "pi-auth.json")
	if err := os.WriteFile(piAuth, []byte(`{"opencode-go":{"type":"api_key","key":"sk-pi"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ocDir := filepath.Join(dir, "opencode")
	if err := os.MkdirAll(ocDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ocAuth := filepath.Join(ocDir, "auth.json")
	if err := os.WriteFile(ocAuth, []byte(`{"opencode":{"type":"api_key","key":"sk-oc"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envKeyVar, "")

	// Flag wins.
	ks, err := resolveKey("sk-flag", piAuth)
	if err != nil || ks.key != "sk-flag" {
		t.Fatalf("flag precedence: %+v err=%v", ks, err)
	}

	// Env wins over auth file.
	t.Setenv(envKeyVar, "sk-env")
	ks, err = resolveKey("", piAuth)
	if err != nil || ks.key != "sk-env" {
		t.Fatalf("env precedence: %+v err=%v", ks, err)
	}
	t.Setenv(envKeyVar, "")

	// pi auth file.
	ks, err = resolveKey("", piAuth)
	if err != nil || ks.key != "sk-pi" {
		t.Fatalf("pi auth: %+v err=%v", ks, err)
	}

	// Missing pi auth -> opencode auth fallback (override via env for test).
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	if err := os.Rename(piAuth, filepath.Join(dir, "pi-auth-backup")); err != nil {
		t.Fatal(err)
	}
	ks, err = resolveKey("", piAuth)
	if err != nil || ks.key != "sk-oc" {
		t.Fatalf("opencode fallback: %+v err=%v", ks, err)
	}

	// Nothing available -> error mentioning the solution.
	if err := os.Remove(ocAuth); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveKey("", piAuth); err == nil {
		t.Fatal("expected error when no key available")
	}
}

func TestMaskKey(t *testing.T) {
	if got := maskKey("sk-abcdefgh"); got != "sk-…efgh" {
		t.Fatalf("maskKey: %q", got)
	}
	if got := maskKey("short"); got != "short" {
		t.Fatalf("maskKey short: %q", got)
	}
}
