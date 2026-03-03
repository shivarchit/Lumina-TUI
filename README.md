# Lumina-TUI

[![CI](https://github.com/shivarchit/Lumina-TUI/actions/workflows/ci.yml/badge.svg)](https://github.com/shivarchit/Lumina-TUI/actions/workflows/ci.yml)

Lumina-TUI is a terminal user interface for controlling WiZ smart lights and plugs locally over UDP.

It is built in Go with Bubble Tea and is intended for fast, local control from the terminal.

---

## Features

- **Zero cloud dependency**  
  Communicates directly with your lights over your local network using UDP port `38899`.  
  No accounts, no cloud, instant response times.

- **24-color visual grid**  
  A fully interactive, responsive grid of curated colors for quick mood setting.

- **Custom hex input**  
  Type any valid hex code (e.g., `#CBA6F7`) to dial in the exact color you want.

- **Visual brightness slider**  
  Adjust dimming levels smoothly using arrow keys or Vim-style navigation.

- **Background sleep timer**  
  Set a timer and watch the animated status spinner run while the UI remains fully interactive.

- **Smart bulb discovery**  
  Auto-scans local subnets, de-duplicates bulbs by MAC/IP, and lets you select and persist a target instantly.

- **Saved device profiles**  
  Save discovered bulbs with custom names and quickly re-select them across app restarts.

- **Live telemetry panel**  
  A btop-inspired dashboard shows command health, latency sparklines, brightness trend, and discovery performance.

- **Clean terminal interface**  
  Multi-pane layout, dynamic border highlights, and a Vim-style bottom status bar (Normal/Insert modes).

---

## Installation

### Prerequisites

- Go 1.18 or higher installed on your machine.

---

### 1) Clone the repository

```bash
git clone https://github.com/shivarchit/Lumina-TUI.git
cd Lumina-TUI
```

---

### 2) Configure your device

Create a `.env` file in the root directory and add your light's local IP address:

```env
WIZ_IP=192.168.1.15
WIZ_PORT=38899
```

> You can find your device's IP address in the WiZ mobile app under **Settings -> Lights**.

---

### 3) Install dependencies

```bash
go mod tidy
```

---

### 4) Run the app

```bash
go run ./internal
```

---

## Build a standalone binary

To use Lumina-TUI without `go run`, compile it into a single executable:

```bash
go build -o lumina ./internal
```

### macOS / Linux

Move the `lumina` binary to `/usr/local/bin` to access it globally:

```bash
sudo mv lumina /usr/local/bin
```

Then simply run:

```bash
lumina
```

### Windows (PowerShell)

Build a Windows executable:

```powershell
go build -o lumina.exe ./internal
```

Create a user bin folder and move the executable there:

```powershell
New-Item -ItemType Directory -Force "$env:USERPROFILE\bin" | Out-Null
Move-Item -Force .\lumina.exe "$env:USERPROFILE\bin\lumina.exe"
```

Add your user bin folder to `PATH` (one-time setup):

```powershell
setx PATH "$env:PATH;$env:USERPROFILE\bin"
```

Close and reopen PowerShell, then run:

```powershell
lumina
```

---

## Controls

The interface is fully keyboard-driven:

- `Up` / `Down` or `k` / `j` - Navigate the menu and color grid  
- `Left` / `Right` or `h` / `l` - Adjust brightness or move horizontally  
- `Enter` - Select / Confirm  
- `r` - Refresh device discovery scan  
- `s` - Save selected discovered device with a custom name  
- `d` - Delete selected saved device  
- `Esc` - Cancel input mode  
- `q` or `Ctrl + C` - Quit application  

---

## Project structure

Lumina-TUI follows a modular architecture:

- `internal/main.go` - CLI entry point  
- `internal/app/run.go` - startup flow and CLI mode handling  
- `internal/config/config.go` - config validation and persistence  
- `internal/ui/` - Bubble Tea model, update loop, and rendering  
- `internal/wiz/client.go` - UDP networking and discovery logic  
- `internal/version/version.go` - application version constant  
- `build/release.sh` - cross-platform release build script  
- `tests/ui/` - UI package black-box tests  
- `tests/wiz/` - WiZ client tests  

---

## Testing

Run all tests:

```bash
go test ./...
```

Run only UI tests:

```bash
go test ./tests/ui/...
```

Run only WiZ client tests:

```bash
go test ./tests/wiz/...
```

---

## CI

GitHub Actions runs tests automatically on push to `main` and on pull requests.

- Workflow file: `.github/workflows/ci.yml`
- Command used in CI: `go test ./...`

---

## Releases

Separate release scripts are available for Unix and Windows.

Unix (macOS/Linux):

```bash
bash build/release.sh
```

Unix dry run (build + package only, no tag push/release upload):

```bash
bash build/release.sh all --dry-run
```

Windows (PowerShell):

```powershell
.\build\release.ps1
```

Windows dry run (build + package only, no tag push/release upload):

```powershell
.\build\release.ps1 -Target all -DryRun
```

Optional single-target build:

- Unix: `bash build/release.sh linux/amd64`
- Windows: `.\build\release.ps1 -Target linux/amd64`

Both scripts:

- build binaries into `dist/`
- generate `checksums.txt`
- create an archive
- ensure/push the git tag from `internal/version/version.go`
- upload assets to GitHub Releases using `gh`

---

## How it works

WiZ devices expose a local UDP API. Lumina-TUI sends structured JSON payloads directly to port `38899`.

Example payload for setting a custom color:

```json
{
  "method": "setPilot",
  "params": {
    "r": 203,
    "g": 166,
    "b": 247,
    "dimming": 100
  }
}
```

---

## Contributing

Contributions are welcome.
Feel free to open an issue or submit a pull request for features such as:

- Multi-light broadcasting  
- Scene support  
- Device auto-discovery  

---

## License

Distributed under the MIT License. See `LICENSE` for more information.