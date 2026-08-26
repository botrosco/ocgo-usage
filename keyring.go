// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// credential is a partial view of an auth entry found in the pi agent or
// opencode CLI auth files. Only the fields we need are decoded; unknown
// fields are ignored.
type credential struct {
	Type   string `json:"type"`
	Key    string `json:"key"`
	Access string `json:"access"`
}

// keySource is a resolved API key together with a human-readable origin
// (used in the header line, e.g. "pi auth: ~/.pi/agent/auth.json -> opencode-go").
type keySource struct {
	key   string
	label string
}

// envKeyVar matches the env var the opencode CLI itself honours.
const envKeyVar = "OPENCODE_API_KEY"

// authEntryNames lists the keys probed (in order) inside the pi agent auth
// file. Entries are only auto-picked when their name starts with "opencode"
// so an unrelated provider credential is never mistaken for a zen key.
var piAuthEntryNames = []string{"opencode-go", "opencode"}

// resolveKey finds an API key using, in order:
//  1. the --api-key flag / OPENCODE_API_KEY env var
//  2. the pi agent auth file (~/.pi/agent/auth.json), entry "opencode-go" then "opencode"
//  3. the opencode CLI auth file (~/.local/share/opencode/auth.json), entry "opencode"
func resolveKey(flagKey, piAuthFile string) (keySource, error) {
	if flagKey != "" {
		return keySource{key: flagKey, label: "flag"}, nil
	}
	if k := os.Getenv(envKeyVar); k != "" {
		return keySource{key: k, label: envKeyVar}, nil
	}

	for _, name := range piAuthEntryNames {
		if ks, ok := lookupAuthFile(piAuthFile, name); ok {
			return ks, nil
		}
	}

	for _, name := range []string{"opencode"} {
		if ks, ok := lookupAuthFile(opencodeAuthPath(), name); ok {
			return ks, nil
		}
	}

	return keySource{}, fmt.Errorf(
		"no API key found: pass --api-key, set %s, or add an %q entry to %s",
		envKeyVar, piAuthEntryNames[0], piAuthFile,
	)
}

// lookupAuthFile reads a JSON auth file and returns the credential stored
// under name (type oauth -> access token; anything else -> key field).
func lookupAuthFile(path, name string) (keySource, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return keySource{}, false
	}
	var all map[string]credential
	if err := json.Unmarshal(data, &all); err != nil {
		return keySource{}, false
	}
	entry, ok := all[name]
	if !ok {
		return keySource{}, false
	}
	key := entry.Key
	if entry.Type == "oauth" {
		key = entry.Access
	}
	if key == "" {
		return keySource{}, false
	}
	return keySource{key: key, label: fmt.Sprintf("%s -> %s", path, name)}, true
}

// piAuthPath returns the pi agent auth file, honouring XDG_CONFIG_HOME.
func piAuthPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "pi", "agent", "auth.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pi", "agent", "auth.json")
}

// opencodeAuthPath returns the opencode CLI auth file, honouring XDG_DATA_HOME.
func opencodeAuthPath() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "opencode", "auth.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "opencode", "auth.json")
}

// maskKey shortens a key for display, e.g. "sk-…WzDYm".
func maskKey(k string) string {
	if len(k) <= 8 {
		return k
	}
	return k[:3] + "…" + k[len(k)-4:]
}

// isOAuthStyle reports whether an auth entry type carries an access token.
func isOAuthStyle(entry credential) bool {
	return strings.EqualFold(entry.Type, "oauth")
}
