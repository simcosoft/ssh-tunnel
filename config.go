package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// configStruct describes the configuration of a single SSH tunnel
type configStruct struct {
	RemoteSSHIP         string `json:"remoteSshIp"`
	RemoteSSHPort       int32  `json:"remoteSshPort"`
	AuthByPwd           bool   `json:"authByPwd"`
	Interactive         bool   `json:"interactive"`
	RemoteUserName      string `json:"remoteUserName"`
	RemotePasswd        string `json:"remotePasswd"`
	PrivateKeyPath      string `json:"sshPrivateKeyPath"`
	PrivateKeyPwd       string `json:"sshPrivateKeyPwd"`
	RemoteTunnelingIP   string `json:"remoteTunnelingIp"`
	RemoteTunnelingPort int32  `json:"remoteTunnelingPort"`
	LocalTunnelingIP    string `json:"localTunnelingIp"`
	LocalTunnelingPort  int32  `json:"localTunnelingPort"`
}

// ConfigStruct holds the list of tunnel configurations loaded from the config file
var ConfigStruct []configStruct

// LoadConfiguration loads tunnel configuration from .ssh_tunnel.json
func LoadConfiguration() error {

	configFile, err := os.Open(".ssh_tunnel.json")
	if err != nil {
		createEmptyConfig()
		return fmt.Errorf("%s. Was created new empty one! ;-)", err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&ConfigStruct)

	if err != nil {
		return fmt.Errorf("error parsing config file: %s", err.Error())
	}

	return nil
}

// ensureGitignore ensures .ssh_tunnel.json is listed in .gitignore,
// creating the file if it does not exist.
func ensureGitignore() {
	const entry = ".ssh_tunnel.json"
	const gitignorePath = ".gitignore"

	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("Failed to read .gitignore: %s\n", err)
		return
	}

	// Check if entry is already present (exact line match)
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			return
		}
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open .gitignore: %s\n", err)
		return
	}
	defer f.Close()

	// Ensure we start on a new line if the file is non-empty and does not end with \n
	if len(data) > 0 && data[len(data)-1] != '\n' {
		fmt.Fprintln(f)
	}
	fmt.Fprintln(f, entry)
}

// createEmptyConfig creates an empty config file with a sample structure
func createEmptyConfig() {
	ensureGitignore()

	f, err := os.OpenFile(".ssh_tunnel.json", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("Failed to create config file: %s", err.Error())
		return
	}
	defer f.Close()

	c := []configStruct{{
		LocalTunnelingIP: "127.0.0.1",
	}}

	out, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		fmt.Printf("Failed to marshal config: %s", err.Error())
		return
	}

	if _, err := f.Write(out); err != nil {
		fmt.Printf("Failed to write config file: %s", err.Error())
	}
}
