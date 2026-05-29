package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// fakeAddr is a minimal net.Addr used in host key callback tests.
type fakeAddr struct{}

func (fakeAddr) Network() string { return "tcp" }
func (fakeAddr) String() string  { return "127.0.0.1:22" }

var _ net.Addr = fakeAddr{}

// newTestPublicKey generates a fresh ed25519 SSH public key for testing.
func newTestPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("create ssh public key: %v", err)
	}
	return sshPub
}

// writeKnownHost appends a single host key entry to the known_hosts file.
func writeKnownHost(t *testing.T, path, hostname string, key ssh.PublicKey) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open known_hosts: %v", err)
	}
	defer f.Close()
	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	if _, err := fmt.Fprintln(f, line); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
}

// pipeStdin replaces os.Stdin with a pipe pre-filled with input and restores
// the original on test cleanup.
func pipeStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := fmt.Fprint(w, input); err != nil {
		t.Fatalf("write pipe: %v", err)
	}
	w.Close()

	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		r.Close()
	})
}

// --- makeHostKeyCallback ---

func TestMakeHostKeyCallback_CreatesKnownHostsFile(t *testing.T) {
	khPath := filepath.Join(t.TempDir(), "known_hosts")

	if _, err := os.Stat(khPath); !os.IsNotExist(err) {
		t.Fatal("known_hosts should not exist before callback creation")
	}

	makeHostKeyCallback(khPath)

	if _, err := os.Stat(khPath); err != nil {
		t.Fatalf("known_hosts was not created: %v", err)
	}
}

func TestMakeHostKeyCallback_KnownHostCorrectKey(t *testing.T) {
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	key := newTestPublicKey(t)
	writeKnownHost(t, khPath, "example.com:22", key)

	cb := makeHostKeyCallback(khPath)
	if err := cb("example.com:22", fakeAddr{}, key); err != nil {
		t.Fatalf("expected nil for known correct key, got: %v", err)
	}
}

func TestMakeHostKeyCallback_KnownHostChangedKey_ReturnsMITMError(t *testing.T) {
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	key1 := newTestPublicKey(t)
	key2 := newTestPublicKey(t)
	writeKnownHost(t, khPath, "example.com:22", key1)

	cb := makeHostKeyCallback(khPath)
	err := cb("example.com:22", fakeAddr{}, key2)
	if err == nil {
		t.Fatal("expected MITM error for changed key, got nil")
	}
	if !strings.Contains(err.Error(), "WARNING") {
		t.Fatalf("expected MITM WARNING in error message, got: %v", err)
	}
}

func TestMakeHostKeyCallback_UnknownHost_UserDeclines(t *testing.T) {
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	key := newTestPublicKey(t)
	pipeStdin(t, "no\n")

	cb := makeHostKeyCallback(khPath)
	err := cb("newhost.example.com:22", fakeAddr{}, key)
	if err == nil {
		t.Fatal("expected error when user declines, got nil")
	}
	if !strings.Contains(err.Error(), "host key verification failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMakeHostKeyCallback_UnknownHost_UserAccepts(t *testing.T) {
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	key := newTestPublicKey(t)
	pipeStdin(t, "yes\n")

	cb := makeHostKeyCallback(khPath)
	if err := cb("newhost.example.com:22", fakeAddr{}, key); err != nil {
		t.Fatalf("expected nil when user accepts, got: %v", err)
	}

	// Key must now be persisted in known_hosts
	data, err := os.ReadFile(khPath)
	if err != nil {
		t.Fatalf("read known_hosts: %v", err)
	}
	if !strings.Contains(string(data), "newhost.example.com") {
		t.Fatalf("key was not saved to known_hosts: %q", string(data))
	}
}

func TestMakeHostKeyCallback_UnknownHost_AcceptedKeyIsReusable(t *testing.T) {
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	key := newTestPublicKey(t)
	pipeStdin(t, "yes\n")

	cb := makeHostKeyCallback(khPath)
	if err := cb("newhost.example.com:22", fakeAddr{}, key); err != nil {
		t.Fatalf("first connect (accept): %v", err)
	}

	// Second connect with the same key must succeed without prompting
	cb2 := makeHostKeyCallback(khPath)
	if err := cb2("newhost.example.com:22", fakeAddr{}, key); err != nil {
		t.Fatalf("second connect (known key): %v", err)
	}
}
