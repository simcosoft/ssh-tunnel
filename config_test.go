package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// withTempDir switches the working directory to a fresh temp dir for the
// duration of the test and restores both the directory and ConfigStruct on
// cleanup. Required for functions that use relative file paths.
func withTempDir(t *testing.T) {
	t.Helper()
	origConfig := ConfigStruct
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(orig)
		ConfigStruct = origConfig
	})
}

// --- LoadConfiguration ---

func TestLoadConfiguration_ValidFile(t *testing.T) {
	withTempDir(t)

	content := `[{"remoteSshIp":"10.0.0.1","remoteSshPort":22,"remoteUserName":"admin"}]`
	if err := os.WriteFile(".ssh_tunnel.json", []byte(content), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := LoadConfiguration(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ConfigStruct) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(ConfigStruct))
	}
	if ConfigStruct[0].RemoteSSHIP != "10.0.0.1" {
		t.Fatalf("expected RemoteSSHIP=10.0.0.1, got %q", ConfigStruct[0].RemoteSSHIP)
	}
}

func TestLoadConfiguration_MultipleEntries(t *testing.T) {
	withTempDir(t)

	content := `[
		{"remoteSshIp":"10.0.0.1","remoteSshPort":22},
		{"remoteSshIp":"10.0.0.2","remoteSshPort":2222}
	]`
	if err := os.WriteFile(".ssh_tunnel.json", []byte(content), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := LoadConfiguration(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ConfigStruct) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(ConfigStruct))
	}
}

func TestLoadConfiguration_InvalidJSON(t *testing.T) {
	withTempDir(t)

	if err := os.WriteFile(".ssh_tunnel.json", []byte("not json {{"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := LoadConfiguration(); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadConfiguration_MissingFileCreatesEmpty(t *testing.T) {
	withTempDir(t)

	err := LoadConfiguration()
	if err == nil {
		t.Fatal("expected error indicating file was missing, got nil")
	}
	if !strings.Contains(err.Error(), "Was created new empty one") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if _, statErr := os.Stat(".ssh_tunnel.json"); statErr != nil {
		t.Fatalf("expected .ssh_tunnel.json to be created: %v", statErr)
	}
}

// --- createEmptyConfig ---

func TestCreateEmptyConfig_ValidJSON(t *testing.T) {
	withTempDir(t)

	createEmptyConfig()

	data, err := os.ReadFile(".ssh_tunnel.json")
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var tunnels []configStruct
	if err := json.Unmarshal(data, &tunnels); err != nil {
		t.Fatalf("created config is not valid JSON: %v", err)
	}
	if len(tunnels) != 1 {
		t.Fatalf("expected 1 template entry, got %d", len(tunnels))
	}
	if tunnels[0].LocalTunnelingIP != "127.0.0.1" {
		t.Fatalf("expected LocalTunnelingIP=127.0.0.1, got %q", tunnels[0].LocalTunnelingIP)
	}
}

func TestCreateEmptyConfig_FilePermissions(t *testing.T) {
	withTempDir(t)

	createEmptyConfig()

	info, err := os.Stat(".ssh_tunnel.json")
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected permissions 0600, got %04o", perm)
	}
}

func TestCreateEmptyConfig_IsIndented(t *testing.T) {
	withTempDir(t)

	createEmptyConfig()

	data, err := os.ReadFile(".ssh_tunnel.json")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "\t") {
		t.Fatal("expected indented JSON (tabs), got compact output")
	}
}

// --- ensureGitignore ---

func TestEnsureGitignore_CreatesFile(t *testing.T) {
	withTempDir(t)

	ensureGitignore()

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".ssh_tunnel.json") {
		t.Fatalf(".gitignore does not contain .ssh_tunnel.json: %q", string(data))
	}
}

func TestEnsureGitignore_DoesNotDuplicate(t *testing.T) {
	withTempDir(t)

	ensureGitignore()
	ensureGitignore()

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if count := strings.Count(string(data), ".ssh_tunnel.json"); count != 1 {
		t.Fatalf("expected exactly 1 occurrence of .ssh_tunnel.json, got %d", count)
	}
}

func TestEnsureGitignore_AppendsToExistingFileWithoutNewline(t *testing.T) {
	withTempDir(t)

	// Existing file without trailing newline
	if err := os.WriteFile(".gitignore", []byte("*.log"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ensureGitignore()

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "*.log") {
		t.Fatal("existing content was lost")
	}
	if !strings.Contains(content, ".ssh_tunnel.json") {
		t.Fatal(".ssh_tunnel.json was not added")
	}
}

func TestEnsureGitignore_AlreadyPresent(t *testing.T) {
	withTempDir(t)

	if err := os.WriteFile(".gitignore", []byte(".ssh_tunnel.json\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ensureGitignore()

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if count := strings.Count(string(data), ".ssh_tunnel.json"); count != 1 {
		t.Fatalf("expected 1 occurrence, got %d", count)
	}
}
