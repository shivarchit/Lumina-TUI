# 💡 Lumina-TUI

A blazing-fast, beautiful Terminal User Interface (TUI) for controlling WiZ smart lights and plugs locally over UDP.

Built in Go with the phenomenal Bubble Tea framework, Lumina-TUI is designed for developers who want instant, cloud-free control of their smart home directly from their terminal.

---

## ✨ Features

- **⚡ Zero Cloud Dependency**  
  Communicates directly with your lights over your local network using UDP port `38899`.  
  No accounts, no cloud, instant response times.

- **🎨 24-Color Visual Grid**  
  A fully interactive, responsive grid of curated colors for quick mood setting.

- **🔧 Custom Hex Input**  
  Type any valid hex code (e.g., `#CBA6F7`) to dial in the exact color you want.

- **🔆 Visual Brightness Slider**  
  Adjust dimming levels smoothly using arrow keys or Vim-style navigation.

- **⏱️ Background Sleep Timer**  
  Set a timer and watch the animated status spinner run while the UI remains fully interactive.

- **✨ Sleek Aesthetic**  
  Clean multi-pane layout, dynamic border highlights, and a Vim-style bottom status bar (Normal/Insert modes).

---

## 📦 Installation

### Prerequisites

- Go 1.18 or higher installed on your machine.

---

### 1️⃣ Clone the Repository

```bash
git clone https://github.com/shivarchit/Lumina-TUI.git
cd Lumina-TUI
```

---

### 2️⃣ Configure Your Device

Create a `.env` file in the root directory and add your light's local IP address:

```env
WIZ_IP=192.168.1.15
WIZ_PORT=38899
```

> You can find your device's IP address in the WiZ mobile app under **Settings → Lights**.

---

### 3️⃣ Install Dependencies

```bash
go mod tidy
```

---

### 4️⃣ Run the App

```bash
go run .
```

---

## 🏗️ Build a Standalone Binary

To use Lumina-TUI without `go run`, compile it into a single executable:

```bash
go build -o lumina
```

Move the `lumina` binary to `/usr/local/bin` to access it globally:

```bash
sudo mv lumina /usr/local/bin
```

Then simply run:

```bash
lumina
```

---

## 🎮 Controls

The interface is fully keyboard-driven:

- `↑` / `↓` or `k` / `j` — Navigate the menu and color grid  
- `←` / `→` or `h` / `l` — Adjust brightness or move horizontally  
- `Enter` — Select / Confirm  
- `Esc` — Cancel input mode  
- `q` or `Ctrl + C` — Quit application  

---

## 🗂️ Project Structure

Lumina-TUI follows a modular architecture:

- `main.go` — Entry point and environment loading  
- `tui.go` — Bubble Tea model, update, and view logic  
- `wiz.go` — UDP networking and JSON payload handling  

---

## ⚙️ How It Works

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

## 🤝 Contributing

Contributions are welcome!  
Feel free to open an issue or submit a pull request for features such as:

- Multi-light broadcasting  
- Scene support  
- Device auto-discovery  

---

## 📜 License

Distributed under the MIT License. See `LICENSE` for more information.