# Heimdall — Gungnir-Style Certificate Transparency Monitor

Heimdall is a modern continuation of the original **Gungnir** workflow: a Go CLI that tails Certificate Transparency logs, writes `Hostname: sub.domain.tld` records per monitored root, and notifies you the moment new certificates appear for your estate. It keeps the beloved Gungnir experience while adding automatic CT log discovery, persistent deduplication, Discord alerts, and hot-reloadable root lists.

---

## Highlights

- **Gungnir-compatible CLI** (`-r`, `-f`, `-o`, `v`, `-j`, `-debug`, `-discord-webhook`) for drop‑in replacement.
- **47 CT logs auto-tracked** from Google’s [official `log_list.v3`](https://www.gstatic.com/ct/log_list/v3/log_list.json) (Argon/Xenon, Cloudflare Nimbus, DigiCert Wyvern/Sphinx, Sectigo Sabre/Mammoth/Elephant/Tiger, Let’s Encrypt Oak, TrustAsia, etc.).
- **Persistent dedupe** — existing `Hostname:` files are read on startup, so historical hits never trigger new alerts.
- **Per-root output directories only** — the path passed to `-o` is always treated as a directory (even if you pass `results.txt`).
- **Discord webhooks** — one embed per newly discovered hostname, already deduplicated.
- **Hot reload** — `-f` monitors your root list and restarts CT workers automatically when that file changes.
- **Future roadmap** — optional httpx/naabu/nmap enrichment, structured storage backends, more transports.

---

## Install

### go install (recommended)
```bash
go install github.com/Imsol0/heimdall/cmd/heimdall@latest
```
Requires Go **1.23+** (matches the same baseline used by `golang.org/x/sys`). The binary appears in `$GOBIN`/`$GOPATH/bin`.

### From source
```bash
git clone https://github.com/Imsol0/heimdall.git
cd heimdall
go build -o heimdall ./cmd/heimdall
```
Ensure your Go toolchain is 1.23 or newer before building or running CI (`actions/setup-go@v4` → `go-version: '1.23'`).

---

## Quick Start
1. Create `domains.txt` with one root per line:
   ```text
   att.com
   tesla.com
   example.com
   ```
2. Run Heimdall:
   ```bash
   heimdall -r domains.txt -f -o output/ \
     -discord-webhook https://discord.com/api/webhooks/XXXX/XXXX
   ```
3. You’ll get:
   ```text
   output/
   ├─ att.com.txt      # lines like “Hostname: lab.domain.att.com”
   ├─ tesla.com.txt
   └─ example.com.txt
   ```
4. Each first-seen hostname is appended once and (optionally) reported to Discord.

Other common modes:
```bash
# Firehose (no filters)
heimdall

# JSON output (Gungnir-style)
heimdall -j -r domains.txt

# Verbose + CT backlog diagnostics
heimdall -r domains.txt -v -debug
```

---

## Flags

| Flag | Description |
|------|-------------|
| `-r <file>` | Plain-text root list. Required when using `-o`. |
| `-f` | Follow mode. Watches the `-r` file and restarts CT readers when the file changes. |
| `-o <dir>` | Directory for per-root output (`<root>.txt`). Always treated as a directory, even if the path ends with `.txt`. |
| `-v` | Verbose logging (network retries, backlog info). |
| `-debug` | Print backlog warnings when CT logs fall behind. |
| `-j` | Emit JSON lines instead of `Hostname:` text on stdout. |
| `-discord-webhook <url>` | Send a Discord embed per newly discovered hostname. |

**Output format:** Each line in `<root>.txt` is `Hostname: sub.domain.tld`. Heimdall deduplicates case-insensitively but preserves the original case on disk and in alerts for compatibility with existing tooling.

---

## How It Works
1. **CT log discovery:** Heimdall downloads [Google’s `log_list.v3`](https://www.gstatic.com/ct/log_list/v3/log_list.json), keeps every “usable / pending / read-only / qualified / retired” log, and discards obvious “bogus / placeholder / example” entries. That yields 47 active logs as of this release.
2. **Fan-in readers:** Each log is tailed by its own goroutine (32-entry batches for Google logs, 256-entry batches for others). `-debug` shows how far behind any log might be.
3. **Filtering & dedupe:** Hostnames are normalized, matched against your root list (if provided), and deduplicated via an in-memory map. The map is hydrated by reading all existing `Hostname:` lines from the output directory on startup.
4. **Persistence & alerts:** First-seen hostnames append to `<root>.txt` and (optionally) fire a Discord webhook. Restarts resume from the last known CT tree size and already-seen hosts.
5. **Hot reload:** With `-f`, Heimdall uses `fsnotify` to watch `domains.txt`; edits trigger a graceful restart of log readers without losing state.

---

## CT Log Coverage
(Auto-updated from Google’s list; no hard-coded arrays.)

- Google Argon 2025/2026/2027 & Xenon 2025/2026/2027
- Cloudflare Nimbus 2025/2026/2027
- DigiCert Yeti, Nessie, Wyvern (’25–’27), Sphinx (’25–’27)
- Sectigo Sabre / Mammoth / Elephant / Tiger cohorts for ’25–’27
- Let’s Encrypt Oak 2025h2 / 2026h1 / 2026h2
- TrustAsia Log2025a/b, log2026a/b, HETU2027

New logs are automatically incorporated as soon as Google marks them usable.

---

## Roadmap

- Optional enrichment via [`httpx`](https://github.com/projectdiscovery/httpx), [`naabu`](https://github.com/projectdiscovery/naabu), and/or `nmap`, with a bounded worker pool so CT ingestion remains real-time.
- Structured storage (SQLite/BoltDB) for teams wanting queryable history instead of flat text files.
- Additional transports (Slack, Teams, Mattermost, generic webhooks).

---

## Release Checklist
Our release pipeline runs:
```bash
go fmt ./...
go vet ./...
go build ./cmd/heimdall
```
…plus a manual end-to-end test (`-r/-f/-o/-discord-webhook`) before tagging.

---

## Credits
Heimdall is inspired by [Gungnir by @g0ldencybersec](https://github.com/g0ldencybersec/gungnir). We’re keeping the spirit alive while enabling deeper automation. Contributions and feature requests are welcome — open an issue or PR at [github.com/Imsol0/heimdall](https://github.com/Imsol0/heimdall).
# Heimdall - CT Log Monitor

Heimdall is a command-line tool written in Go that continuously monitors Certificate Transparency (CT) logs for newly issued SSL/TLS certificates. It's designed to work exactly like Gungnir with the same CLI interface and behavior.

## Installation

```bash
go build -o heimdall ./cmd/heimdall
```

## Usage

```bash
# Filtered mode (requires root domains file)
./heimdall -r domains.txt

# Filtered mode with file or directory output
./heimdall -r domains.txt -o results/
# or write everything into a single file
./heimdall -r domains.txt -o results.txt

# Unfiltered mode (monitors everything)
./heimdall

# Verbose mode
./heimdall -r domains.txt -v

# JSON output
./heimdall -r domains.txt -j
# Follow mode (reload when domains.txt changes)
./heimdall -r domains.txt -f -o results/
```

## Options

- `-r`: Path to the list of root domains to filter against
- `-o`: Directory path (required with `-r`). Heimdall always creates one `<root>.txt` file per domain inside that directory—even if you pass something like `results.txt`, it will be treated as a directory name.
- `-f`: Monitor the root domain file for updates and restart the scan (requires -r flag)
- `-discord-webhook`: Discord webhook URL for instant notifications
- `-v`: Output go logs (500/429 errors) to command line
- `-debug`: Debug CT logs to see if you are keeping up
- `-j`: JSONL output cert info

## Example

Create a `domains.txt` file:
```
att.com
tesla.com
example.com
```

Run:
```bash
./heimdall -r domains.txt -o results/ -discord-webhook https://discord.com/api/webhooks/...
```

This will create:
- `results/att.com.txt` - Contains entries like `Hostname: lab.domain.att.com`
- `results/tesla.com.txt`
- `results/example.com.txt`

## How It Works

Heimdall automatically connects to ALL active CT logs from Google's official log list. It monitors them in real-time and extracts domains from certificates as they are issued. When running with `-r` and `-o`, it writes new domains either into per-root files (directory mode) or a single file (when you pass a `.txt` path), exactly like Gungnir. With `-f`, Heimdall watches your `domains.txt` for changes and restarts scanning automatically. When `-discord-webhook` is provided, each new domain is deduplicated and delivered instantly to your Discord channel.

