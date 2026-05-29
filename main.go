package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func main() {

	// Show the current version:
	log.Println(`SSHTunnel v1.4.0`)

	err := LoadConfiguration()
	if err != nil {
		log.Printf("Load configuration error: %s", err.Error())
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %s", err.Error())
	}

	// Collect passwords interactively before starting tunnels.
	// Done sequentially here so prompts don't interleave.
	for i := range ConfigStruct {
		cnf := &ConfigStruct[i]
		if !cnf.Interactive {
			continue
		}
		if cnf.AuthByPwd {
			fmt.Printf("Tunnel %s:%d — SSH password for %s: ", cnf.RemoteSSHIP, cnf.RemoteSSHPort, cnf.RemoteUserName)
			pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				log.Fatalf("Failed to read password: %s", err)
			}
			cnf.RemotePasswd = string(pwd)
		} else {
			keyPath := cnf.PrivateKeyPath
			if keyPath == "" {
				keyPath = "~/.ssh/id_rsa"
			}
			fmt.Printf("Tunnel %s:%d — passphrase for key %s: ", cnf.RemoteSSHIP, cnf.RemoteSSHPort, keyPath)
			pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				log.Fatalf("Failed to read passphrase: %s", err)
			}
			cnf.PrivateKeyPwd = string(pwd)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(ConfigStruct))

	for _, cnf := range ConfigStruct {
		go runTunnel(cnf, homeDir, &wg)
	}

	wg.Wait()
}

func runTunnel(cnf configStruct, homeDir string, wg *sync.WaitGroup) {
	defer wg.Done()

	config := &ssh.ClientConfig{
		User:            cnf.RemoteUserName,
		HostKeyCallback: makeHostKeyCallback(filepath.Join(homeDir, ".ssh", "known_hosts")),
		Timeout:         30 * time.Second,
	}

	if cnf.AuthByPwd {
		config.Auth = []ssh.AuthMethod{
			ssh.Password(cnf.RemotePasswd),
			ssh.PasswordCallback(passwordCallback(cnf.RemotePasswd)),
			ssh.KeyboardInteractive(keyboardInteractiveChallenge(cnf.RemotePasswd)),
		}
	} else {
		var pubKeyPath string
		if cnf.PrivateKeyPath == "" {
			pubKeyPath = filepath.Join(homeDir, ".ssh", "id_rsa")
		} else {
			pubKeyPath = cnf.PrivateKeyPath
		}

		pubKeyBytes, err := os.ReadFile(pubKeyPath)
		if err != nil {
			log.Fatalf("Failed to read private key file: %s", err.Error())
		}

		pubKey, err := ssh.ParsePrivateKeyWithPassphrase(pubKeyBytes, []byte(cnf.PrivateKeyPwd))
		if err != nil {
			log.Fatalf("Failed to parse private key: %s", err.Error())
		}

		config.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(pubKey),
		}
	}

	// Bind local end-point:
	localListener := createLocalEndPoint(fmt.Sprintf("%s:%d", cnf.LocalTunnelingIP, cnf.LocalTunnelingPort))

	// Accept client connections (blocks forever):
	acceptClients(
		localListener,
		config,
		fmt.Sprintf("%s:%d", cnf.RemoteSSHIP, cnf.RemoteSSHPort),
		fmt.Sprintf("%s:%d", cnf.RemoteTunnelingIP, cnf.RemoteTunnelingPort))
}
