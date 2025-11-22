# Heimdall — Certificate Transparency Monitor

Heimdall is a Go-powered CLI that keeps watch over public Certificate Transparency (CT) logs, writes new hostnames to per‑root files, and can alert you instantly through Discord. It is compatible with the formats and automation the CT community already uses, while providing modern quality-of-life upgrades.

---

## What You Get
- **Real-time CT ingestion** from 47 actively maintained logs (Argon/Xenon, Cloudflare Nimbus, DigiCert Wyvern/Sphinx, Sectigo Sabre/Mammoth/Elephant/Tiger, Let’s Encrypt Oak, TrustAsia, etc.).
- **Per-root outputs only** – you point `-o` at a directory and Heimdall creates `<root>.txt` files containing `Hostname: sub.domain.tld` lines. Even if you pass `-o results.txt`, it becomes a directory named `results.txt/` so your archives stay organized.
- **Persistent dedupe** – Heimdall re-reads every stored `Hostname:` line on startup, so you never get duplicate alerts after a restart.
- **Drop-in CLI** – flags like `-r`, `-f`, `-o`, `-j`, `-debug`, `-discord-webhook` make it easy to integrate into existing pipelines.
- **Hot reload** – `-f` watches your root list for changes and restarts log readers automatically.
- **Discord notifications** – optional single-line embeds per new hostname.
- **Roadmap** – planned optional enrichment via httpx/naabu/nmap, structured storage, and more transports (Slack, Teams, etc.).

---

## Installation

### go install (recommended)
```bash
go install github.com/Imsol0/heimdall/cmd/heimdall@latest
```
Requires Go **1.23+**. The binary ends up in `$GOBIN` or `$GOPATH/bin`.

### From source
```bash
git clone https://github.com/Imsol0/heimdall.git
cd heimdall
go build -o heimdall ./cmd/heimdall
```
Make sure your toolchain (and CI) uses Go ≥ 1.23. Example GitHub Actions snippet:
```yaml
- uses: actions/setup-go@v4
  with:
    go-version: '1.23'
```

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
3. Directory layout:
   ```text
   output/
   ├─ att.com.txt      # entries like “Hostname: lab.domain.att.com”
   ├─ tesla.com.txt
   └─ example.com.txt
   ```
4. Every first-seen hostname is appended once and (optionally) posted to Discord.

Other useful modes:
```bash
# Firehose: monitor every hostname in every CT log
heimdall

# JSON output for log shipping
heimdall -j -r domains.txt

# Verbose + backlog diagnostics
heimdall -r domains.txt -v -debug
```

---

## CLI Flags
| Flag | Description |
|------|-------------|
| `-r <file>` | Plain-text list of root domains (required when using `-o`). |
| `-f` | Follow mode – watches the root file and restarts CT readers automatically. |
| `-o <dir>` | Directory for per-root output (`<root>.txt`). Always treated as a directory even if the path looks like a file. |
| `-v` | Verbose logging. |
| `-debug` | Print per-log backlog info to track CT delays. |
| `-j` | Emit JSON instead of `Hostname:` lines on stdout. |
| `-discord-webhook <url>` | Send a Discord embed per newly discovered hostname. |

**Output format:** each line in `<root>.txt` is `Hostname: sub.domain.tld`. Heimdall deduplicates case-insensitively but preserves source casing in the file and alert.

---

## How It Works
1. **CT log discovery** – on startup Heimdall downloads Google’s `log_list.v3`, keeps every usable/pending/read-only/qualified/retired log, and ignores obvious bogus/test entries. That currently yields 47 sources.
2. **Fan-in readers** – each log runs in its own goroutine (Google logs: 32-entry batches; others: 256-entry batches). `-debug` shows the backlog per log.
3. **Filtering & dedupe** – hostnames are normalized, matched against your root list (if provided), and deduped via an in-memory map seeded from existing output files.
4. **Persistence & alerts** – first-time hits append to `<root>.txt` and optionally trigger a Discord webhook.
5. **Hot reload** – enabling `-f` wires up `fsnotify`; editing `domains.txt` triggers a graceful restart of log readers without manual intervention.

---

## Tracking Scope
Heimdall automatically tails the CT logs below (and any new ones Google promotes in the future):

- **Google**: Argon 2025/2026/2027, Xenon 2025/2026/2027
- **Cloudflare**: Nimbus 2025/2026/2027
- **DigiCert**: Yeti, Nessie, Wyvern ’25–’27, Sphinx ’25–’27
- **Sectigo**: Sabre/Mammoth/Elephant/Tiger (2025–2027 cohorts)
- **Let’s Encrypt**: Oak 2025h2 / 2026h1 / 2026h2
- **TrustAsia**: Log2025a/b, Log2026a/b, HETU2027

Because Heimdall reads `log_list.v3` at runtime, you automatically inherit future CT logs as the ecosystem expands.

---

## Pre-release Checklist
Before tagging or releasing, run:
```bash
go fmt ./...
go vet ./...
go build ./cmd/heimdall
```
…and do a manual end-to-end check (`-r/-f/-o/-discord-webhook`) to verify dedupe + alerts.

---

## Roadmap
- Optional httpx/naabu/nmap enrichment (with bounded worker pools so CT ingestion remains real-time).
- Structured storage options (SQLite/BoltDB) for teams that prefer queryable history.
- Additional transports (Slack, Teams, Mattermost, custom webhooks).

Ideas and contributions are welcome — open an issue or PR at [github.com/Imsol0/heimdall](https://github.com/Imsol0/heimdall). মনে রাখবেন: keep it fast, keep it clean, and keep every certificate in sight.
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

