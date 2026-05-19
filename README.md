# File Transfer (ftr)

`ftr` is a simple **LAN file transfer tool** written in Go.  
It discovers peers automatically on the same Wi-Fi/Ethernet network (via mDNS/Bonjour) and transfers files or folders directly from the command line — no relay servers, no GUIs, just pure terminal.

---

## Features

- 🚀 Pure CLI workflow
- 🌐 Zero-config peer discovery (mDNS/Bonjour)
- 📂 Send files or whole directories
- 📥 Receivers store files into `~/Downloads` by default
- ⚙️ Override receive directory with `--dropdir`
- 🔑 Optional pre-shared key (`--key`) for simple authentication
- 🐧 Cross-platform: Linux & macOS (Windows support planned)

---

## Install

Clone and build with Go:

```bash
git clone https://github.com/yourname/ftr.git
cd ftr
go build -o ftr .
````

---

## Usage

Start a background receiver on each machine:

```bash
ftr join --dropdir ~/Downloads --key 123456
```

List available peers on the LAN:

```bash
ftr list
# alice-mac   192.168.1.12  port=8844  dropbox=/Users/alice/Downloads
# bob-linux   192.168.1.23  port=8844  dropbox=/home/bob/Downloads
```

Send a file or directory:

```bash
ftr send --key secret123 ./file.txt alice-mac
ftr send --key secret123 ./myfolder bob-linux
```

---

## Command Reference

### `ftr join`

Start the receiver and advertise presence.

Flags:

* `--dropdir <dir>`  (default `~/Downloads`)
* `--port <n>`           (default `8844`)
* `--key <key>`          (optional, require a passkey for transfers)

### `ftr list`

Show all peers discovered via mDNS.

### `ftr send --key <key> <path> <peer>`

Send a file or directory to a peer.

---

## How It Works

* **Discovery:** Uses mDNS/Bonjour to advertise `_ftr._tcp.local` service on LAN.
* **Transfer:** Simple HTTP endpoint `/upload`, streams tar+gzip archive.
* **Auth:** If `--key` is set, sender must provide matching key (`X-Ftr-Passkey: <key>`).
* **Storage:** Files extracted into the receiver’s dropbox directory.
