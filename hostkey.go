package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var (
	knownHostsWriteMu sync.Mutex
	stdinMu           sync.Mutex
)

// makeHostKeyCallback returns a HostKeyCallback that behaves like the standard SSH client:
// - known host with a matching key  — connection allowed
// - known host with a changed key   — connection rejected (possible MITM attack)
// - unknown host                    — interactive prompt; on confirmation the key is saved to known_hosts
func makeHostKeyCallback(knownHostsPath string) ssh.HostKeyCallback {
	// Create the known_hosts file if it does not exist yet
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		if f, err := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_WRONLY, 0600); err == nil {
			f.Close()
		}
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// Re-read known_hosts on every call so that keys added by
		// other goroutines during runtime are visible
		cb, err := knownhosts.New(knownHostsPath)
		if err != nil {
			// Extract the line number from the error string.
			// knownhosts error format: "knownhosts: <path>:<line>: <reason>"
			lineNum := "unknown"
			if after, ok := strings.CutPrefix(err.Error(), "knownhosts: "+knownHostsPath+":"); ok {
				if idx := strings.Index(after, ":"); idx > 0 {
					lineNum = after[:idx]
				}
			}
			log.Printf("ERROR: %s appears to be corrupted at line %s and cannot be parsed: %v", knownHostsPath, lineNum, err)
			log.Printf("To fix, inspect line %s of the file and remove or repair the invalid entry, then restart.", lineNum)
			return fmt.Errorf("known_hosts corrupted (%s): %w", knownHostsPath, err)
		}

		err = cb(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}

		// Host is known but the key has changed — possible MITM attack
		if len(keyErr.Want) > 0 {
			return fmt.Errorf(
				"@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n"+
					"@ WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED! @\n"+
					"@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n"+
					"IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!\n"+
					"Remote host key for '%s' has changed.\n"+
					"Remove the old key from known_hosts to proceed.",
				hostname,
			)
		}

		// Unknown host — prompt the user (one prompt at a time)
		fingerprint := ssh.FingerprintSHA256(key)

		stdinMu.Lock()
		fmt.Printf("\nThe authenticity of host '%s' can't be established.\n", hostname)
		fmt.Printf("%s key fingerprint is %s.\n", key.Type(), fingerprint)
		fmt.Print("Are you sure you want to continue connecting (yes/no)? ")

		scanner := bufio.NewScanner(os.Stdin)
		var answer string
		if scanner.Scan() {
			answer = strings.TrimSpace(scanner.Text())
		}
		stdinMu.Unlock()

		if answer != "yes" {
			return fmt.Errorf("host key verification failed for %s", hostname)
		}

		// Append the key to known_hosts
		knownHostsWriteMu.Lock()
		defer knownHostsWriteMu.Unlock()

		f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return fmt.Errorf("failed to open known_hosts for writing: %w", err)
		}
		defer f.Close()

		// Ensure the new entry starts on its own line even if the file
		// does not end with a newline (guards against pre-existing corruption).
		if existing, err := os.ReadFile(knownHostsPath); err == nil {
			if len(existing) > 0 && existing[len(existing)-1] != '\n' {
				fmt.Fprintln(f)
			}
		}

		line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("failed to write to known_hosts: %w", err)
		}

		log.Printf("Permanently added '%s' (%s) to the list of known hosts.", hostname, key.Type())
		return nil
	}
}
