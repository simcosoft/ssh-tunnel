# ssh_tunnel

A lightweight CLI tool that keeps multiple persistent SSH tunnels alive, configured via a single JSON file.

**Typical use cases:**

- **Local development against remote services** — expose a production or staging database, cache, or message broker on a local port so your app connects as if the service were local
- **Accessing services in private networks** — reach hosts that have no public IP by jumping through a bastion server
- **Bypassing firewall restrictions** — forward a blocked port through an allowed SSH connection
- **Securing unencrypted protocols** — wrap Redis, PostgreSQL, HTTP, and other plaintext services in an encrypted SSH channel
- **Multi-tunnel management** — keep all your tunnels described in one config file and start them all with a single command instead of juggling multiple terminal tabs

## ✨ Features

- Run any number of tunnels concurrently
- Password authentication and private key (with optional passphrase) authentication
- `interactive` mode — passwords are prompted at startup and never stored on disk
- Host key verification matching the standard `ssh` client behaviour:
  - Known host with a correct key → allowed silently
  - Known host with a **changed** key → rejected with a MITM warning
  - Unknown host → interactive fingerprint prompt; accepted key is saved to `~/.ssh/known_hosts`
- Automatic retry on connection failure (up to 16 attempts)
- Cross-platform build via `Makefile` (Unix / Git Bash / Windows CMD)

## 📋 Requirements

- Go 1.25 or later

## 🚀 Installation

```bash
# Clone the repository
git clone https://github.com/SimCoSoft/ssh_tunnel.git
cd ssh_tunnel
```

```bash
# Build for the current platform
make build

# Build and install to /usr/local/bin (Unix) or C:\Windows\System32 (Windows)
make install

# Cross-compile for Windows from Unix
make build-windows
```

## ⚙️ Configuration

On first run, if `.ssh_tunnel.json` is not found, the tool creates it with an empty template and adds `.ssh_tunnel.json` to `.gitignore` automatically.

The config file is an array of tunnel objects:

```json
[
  {
    "remoteSshIp":         "bastion.example.com",
    "remoteSshPort":       22,
    "remoteUserName":      "alice",

    "authByPwd":           false,
    "interactive":         true,
    "remotePasswd":        "",
    "sshPrivateKeyPath":   "",
    "sshPrivateKeyPwd":    "",

    "remoteTunnelingIp":   "db.internal",
    "remoteTunnelingPort": 5432,
    "localTunnelingIp":    "127.0.0.1",
    "localTunnelingPort":  15432
  },
  ...
]
```

### 🗂️ Configuration fields

| Field                 | Type   | Description |
|-----------------------|--------|-------------|
| `remoteSshIp`         | string | SSH server hostname or IP |
| `remoteSshPort`       | int    | SSH server port (usually `22`) |
| `remoteUserName`      | string | SSH login username |
| `authByPwd`           | bool   | `true` — password auth; `false` — key-based auth |
| `interactive`         | bool   | If `true`, passwords/passphrases are prompted at startup (see below) |
| `remotePasswd`        | string | SSH password (used when `authByPwd: true` and `interactive: false`) |
| `sshPrivateKeyPath`   | string | Path to private key file. Defaults to `~/.ssh/id_rsa` when empty |
| `sshPrivateKeyPwd`    | string | Passphrase for the private key (used when `interactive: false`) |
| `remoteTunnelingIp`   | string | Hostname/IP of the service to reach through the tunnel |
| `remoteTunnelingPort` | int    | Port of the remote service |
| `localTunnelingIp`    | string | Local bind address (e.g. `127.0.0.1`) |
| `localTunnelingPort`  | int    | Local port to listen on |

The tunnel forwards traffic as:

```
localhost:<localTunnelingPort>  →  <remoteSshIp>:<remoteSshPort>  →  <remoteTunnelingIp>:<remoteTunnelingPort>
```

### 🔐 Interactive mode

When `"interactive": true`, the tool ignores any passwords stored in the config and prompts the user at startup **before** any tunnel is started. Input is read without echo (like `sudo`).

```
Tunnel bastion.example.com:22 — SSH password for alice:
```

This is the recommended approach — it avoids storing credentials in the config file at all.

## 🖥️ Using

### First run

Run the tool without any arguments from the directory that contains (or should contain) your `.ssh_tunnel.json` config file:

```bash
ssh_tunnel
```

If the config file does not exist, `ssh_tunnel` creates an empty `.ssh_tunnel.json` template in the **current working directory** and exits:

```
Config file was not found. Was created new empty one: .ssh_tunnel.json
```

Open `.ssh_tunnel.json`, fill in your tunnel definitions, and run the tool again.

### Starting tunnels

```bash
ssh_tunnel
```

The tool reads `.ssh_tunnel.json` from the **current working directory** and starts all tunnels concurrently. It runs in the foreground — press `Ctrl+C` to stop.

For tunnels with `"interactive": true`, you will be prompted for credentials before any connection is made:

```
Tunnel bastion.example.com:22 — SSH password for alice:
```

> **Tip:** It is convenient to run `ssh_tunnel` in a dedicated terminal window or in a separate console tab of your IDE — this keeps tunnel output and interactive prompts out of your main working terminal.

### New host fingerprint prompt

The first time you connect to a host that is not yet in `~/.ssh/known_hosts`, you will see:

```
Unknown host bastion.example.com:22
Fingerprint: SHA256:abc123...
Add to known hosts? (yes/no):
```

Type `yes` to trust the host and save its key. Any other input aborts the connection.

### Connection retries

If a tunnel drops, `ssh_tunnel` retries the connection automatically (up to 16 attempts with a short back-off). You will see log lines like:

```
2026/05/29 12:00:01 [bastion.example.com:22] dial attempt 2/16 failed: ...
```

Once all retry attempts are exhausted, the tunnel goroutine exits; the remaining tunnels keep running.

### Running as a background service (not recommended)

On macOS / Linux, you can keep `ssh_tunnel` running after you close the terminal:

```bash
nohup ssh_tunnel > ~/.ssh_tunnel.log 2>&1 &
```

Or create a `systemd` / `launchd` service pointing to the binary and the working directory that holds `.ssh_tunnel.json`.

## 🛡️ Security notes

> **Warning:** This tool is strictly not recommended for use in production environments.

- Store `.ssh_tunnel.json` outside of version control (the tool sets this up automatically via `.gitignore`).
- Use `"interactive": true` and leave password fields empty to avoid credentials on disk.
- Host key verification is enabled by default and uses `~/.ssh/known_hosts`.

## 🛠️ Development

```bash
# Run all tests
make test

# Remove build artifacts
make clean
```

## 📄 License

MIT
