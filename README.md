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
- `results/att.com.txt` - Contains all att.com subdomains
- `results/tesla.com.txt` - Contains all tesla.com subdomains
- `results/example.com.txt` - Contains all example.com subdomains

## How It Works

Heimdall automatically connects to ALL active CT logs from Google's official log list. It monitors them in real-time and extracts domains from certificates as they are issued. When running with `-r` and `-o`, it writes new domains either into per-root files (directory mode) or a single file (when you pass a `.txt` path), exactly like Gungnir. With `-f`, Heimdall watches your `domains.txt` for changes and restarts scanning automatically. When `-discord-webhook` is provided, each new domain is deduplicated and delivered instantly to your Discord channel.

