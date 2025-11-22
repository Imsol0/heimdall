package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Imsol0/heimdall/pkg/notify"
	"github.com/Imsol0/heimdall/pkg/types"
	"github.com/Imsol0/heimdall/pkg/utils"
	"github.com/fsnotify/fsnotify"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/x509"
)

var (
	logListURL       = "https://www.gstatic.com/ct/log_list/v3/all_logs_list.json"
	defaultRateLimit = map[string]time.Duration{
		"Google":        time.Millisecond,
		"Sectigo":       4 * time.Second,
		"Let's Encrypt": time.Second,
		"DigiCert":      time.Second,
		"TrustAsia":     time.Second,
		"Cloudflare":    time.Second,
	}
)

type Runner struct {
	options    *Options
	logClients []types.CtLog

	rootMu      sync.RWMutex
	rootDomains map[string]bool

	rateLimitMap   map[string]time.Duration
	entryTasksChan chan types.EntryTask

	outputPath  string
	outputMutex sync.Mutex

	seenMu     sync.RWMutex
	seenDomain map[string]bool

	notifier *notify.Notifier

	watcher     *fsnotify.Watcher
	restartChan chan struct{}
}

func NewRunner(options *Options) (*Runner, error) {
	outputPath := strings.TrimSpace(options.OutputDir)
	if outputPath != "" && options.RootList == "" {
		return nil, fmt.Errorf("the -o flag requires the -r flag to be set")
	}

	r := &Runner{
		options:      options,
		rootDomains:  make(map[string]bool),
		seenDomain:   make(map[string]bool),
		rateLimitMap: defaultRateLimit,
		outputPath:   outputPath,
		restartChan:  nil,
	}

	if err := r.loadRootDomains(); err != nil {
		return nil, fmt.Errorf("failed to load root domains: %w", err)
	}

	if r.outputPath != "" {
		if err := os.MkdirAll(r.outputPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := r.hydrateExistingResults(); err != nil {
			return nil, fmt.Errorf("failed to hydrate existing results: %w", err)
		}
	}

	var err error
	r.logClients, err = utils.PopulateLogs(logListURL)
	if err != nil {
		return nil, fmt.Errorf("failed to populate CT logs: %w", err)
	}

	if options.DiscordWebhook != "" {
		r.notifier = notify.New(options.DiscordWebhook)
		if r.notifier != nil {
			log.Printf("[+] Discord notifications enabled")
		}
	}

	if options.WatchFile {
		if err := r.setupWatcher(); err != nil {
			return nil, err
		}
	}

	log.Printf("[*] Initializing All CT logs", len(r.logClients))
	r.entryTasksChan = make(chan types.EntryTask, len(r.logClients)*100)

	return r, nil
}

func (r *Runner) Run() {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	r.startScan(ctx, &wg)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	restarts := r.restartChannel()

	for {
		select {
		case <-signals:
			fmt.Println("[*] Shutting down...")
			cancel()
			<-done
			r.closeWatcher()
			return
		case <-done:
			r.closeWatcher()
			return
		case <-restarts:
			fmt.Println("[*] Root domain file updated. Restarting scan...")
			cancel()
			<-done
			ctx, cancel = context.WithCancel(context.Background())
			wg = sync.WaitGroup{}
			done = make(chan struct{})
			r.startScan(ctx, &wg)
			go func() {
				wg.Wait()
				close(done)
			}()
		}
	}
}

func (r *Runner) startScan(ctx context.Context, wg *sync.WaitGroup) {
	for i := 0; i < len(r.logClients); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.entryWorker(ctx)
		}()
	}

	for _, ctl := range r.logClients {
		wg.Add(1)
		go r.scanLog(ctx, ctl, wg)
	}
}

func (r *Runner) entryWorker(ctx context.Context) {
	for {
		select {
		case task, ok := <-r.entryTasksChan:
			if !ok {
				return
			}
			r.processEntries(task.Entries, task.Index)
		case <-ctx.Done():
			return
		}
	}
}

func (r *Runner) scanLog(ctx context.Context, ctl types.CtLog, wg *sync.WaitGroup) {
	defer wg.Done()

	tickerDuration := time.Second
	for key, val := range r.rateLimitMap {
		if strings.Contains(ctl.Name, key) {
			tickerDuration = val
			break
		}
	}

	isGoogle := strings.Contains(ctl.Name, "Google")
	ticker := time.NewTicker(tickerDuration)
	defer ticker.Stop()

	var start, end int64
	var err error

	for retries := 0; retries < 3; retries++ {
		if err = r.fetchAndUpdateSTH(ctx, ctl, &end); err != nil {
			if r.options.Verbose {
				fmt.Fprintf(os.Stderr, "Retry %d: failed to get STH for %s: %v\n", retries+1, ctl.Client.BaseURI(), err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		break
	}

	start = end - 20

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if start >= end {
				if err = r.fetchAndUpdateSTH(ctx, ctl, &end); err != nil {
					if r.options.Verbose {
						fmt.Fprintf(os.Stderr, "Failed to update STH for %s: %v\n", ctl.Name, err)
					}
					continue
				}
				if r.options.Debug && end-start > 25 {
					fmt.Fprintf(os.Stderr, "%s is behind by %d entries\n", ctl.Name, end-start)
				}
				continue
			}

			if isGoogle {
				for start < end {
					batchEnd := start + 32
					if batchEnd > end {
						batchEnd = end
					}

					entries, err := ctl.Client.GetRawEntries(ctx, start, batchEnd)
					if err != nil {
						if r.options.Verbose {
							fmt.Fprintf(os.Stderr, "Error fetching entries for %s: %v\n", ctl.Name, err)
						}
						break
					}

					if len(entries.Entries) == 0 {
						break
					}

					r.entryTasksChan <- types.EntryTask{Entries: entries, Index: start}
					start += int64(len(entries.Entries))
				}
				continue
			}

			entries, err := ctl.Client.GetRawEntries(ctx, start, end)
			if err != nil {
				if r.options.Verbose {
					fmt.Fprintf(os.Stderr, "Error fetching entries for %s: %v\n", ctl.Name, err)
				}
				continue
			}

			if len(entries.Entries) > 0 {
				r.entryTasksChan <- types.EntryTask{Entries: entries, Index: start}
				start += int64(len(entries.Entries))
			}
		}
	}
}

func (r *Runner) fetchAndUpdateSTH(ctx context.Context, ctl types.CtLog, end *int64) error {
	wsth, err := ctl.Client.GetSTH(ctx)
	if err != nil {
		return err
	}
	*end = int64(wsth.TreeSize)
	return nil
}

func (r *Runner) processEntries(results *ct.GetEntriesResponse, start int64) {
	index := start

	for _, entry := range results.Entries {
		index++
		rle, err := ct.RawLogEntryFromLeaf(index, &entry)
		if err != nil {
			if r.options.Verbose {
				fmt.Fprintf(os.Stderr, "Failed to parse entry %d: %v\n", index, err)
			}
			continue
		}

		switch rle.Leaf.TimestampedEntry.EntryType {
		case ct.X509LogEntryType:
			r.logCertInfo(rle)
		case ct.PrecertLogEntryType:
			r.logPrecertInfo(rle)
		}
	}
}

func (r *Runner) logCertInfo(entry *ct.RawLogEntry) {
	parsed, err := entry.ToLogEntry()
	if x509.IsFatal(err) || parsed.X509Cert == nil {
		if r.options.Verbose {
			log.Printf("Error parsing cert at index %d: %v", entry.Index, err)
		}
		return
	}

	if parsed.X509Cert.Subject.CommonName != "" {
		r.processDomain(parsed.X509Cert.Subject.CommonName, parsed.X509Cert)
	}
	for _, domain := range parsed.X509Cert.DNSNames {
		r.processDomain(domain, parsed.X509Cert)
	}
}

func (r *Runner) logPrecertInfo(entry *ct.RawLogEntry) {
	parsed, err := entry.ToLogEntry()
	if x509.IsFatal(err) || parsed.Precert == nil {
		if r.options.Verbose {
			log.Printf("Error parsing precert at index %d: %v", entry.Index, err)
		}
		return
	}

	if parsed.Precert.TBSCertificate.Subject.CommonName != "" {
		r.processDomain(parsed.Precert.TBSCertificate.Subject.CommonName, parsed.Precert.TBSCertificate)
	}
	for _, domain := range parsed.Precert.TBSCertificate.DNSNames {
		r.processDomain(domain, parsed.Precert.TBSCertificate)
	}
}

func (r *Runner) processDomain(domain string, cert interface{}) {
	display := strings.TrimSpace(domain)
	if display == "" {
		return
	}
	normalized := strings.ToLower(display)

	hasRoots := r.hasRootFilter()
	var matchedRoot string
	if hasRoots {
		var ok bool
		matchedRoot, ok = r.matchingRoot(normalized)
		if !ok {
			return
		}
	}

	if !r.markSeen(normalized) {
		return
	}

	r.emitStdout(display, cert)

	if r.outputPath != "" {
		if _, err := r.persistDomain(display, matchedRoot); err != nil && r.options.Verbose {
			log.Printf("Error writing domain %s: %v", display, err)
		}
	}

	r.notifyDomain(display)
}

func (r *Runner) emitStdout(domain string, cert interface{}) {
	if r.options.JsonOutput {
		switch c := cert.(type) {
		case *x509.Certificate:
			utils.JsonOutput(c)
		default:
			fmt.Println(domain)
		}
		return
	}

	if r.options.OutputDir == "" || !r.hasRootFilter() {
		fmt.Println(domain)
		return
	}

	if r.hasRootFilter() {
		fmt.Println(domain)
	}
}

func (r *Runner) persistDomain(domain string, matchedRoot string) (bool, error) {
	if r.outputPath == "" {
		return false, nil
	}

	if matchedRoot == "" {
		return false, nil
	}
	if err := os.MkdirAll(r.outputPath, 0755); err != nil {
		return false, err
	}

	fileName := strings.ReplaceAll(matchedRoot, ":", "_") + ".txt"
	targetPath := filepath.Join(r.outputPath, fileName)

	r.outputMutex.Lock()
	defer r.outputMutex.Unlock()

	f, err := os.OpenFile(targetPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()

	line := fmt.Sprintf("Hostname: %s\n", domain)
	if _, err := f.WriteString(line); err != nil {
		return false, err
	}

	return true, nil
}

func (r *Runner) hydrateExistingResults() error {
	if r.outputPath == "" {
		return nil
	}

	entries, err := os.ReadDir(r.outputPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	total := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		count, err := r.loadResultsFromFile(filepath.Join(r.outputPath, entry.Name()))
		if err != nil {
			return err
		}
		total += count
	}

	if total > 0 {
		log.Printf("[*] Hydrated %d existing domains from %s", total, r.outputPath)
	}
	return nil
}

func (r *Runner) loadResultsFromFile(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "hostname:") {
			line = strings.TrimSpace(line[len("Hostname:"):])
		}

		if line == "" {
			continue
		}

		norm := strings.ToLower(line)
		r.seenDomain[norm] = true
		count++
	}
	if err := scanner.Err(); err != nil {
		return count, err
	}
	return count, nil
}

func (r *Runner) markSeen(domain string) bool {
	r.seenMu.RLock()
	if r.seenDomain[domain] {
		r.seenMu.RUnlock()
		return false
	}
	r.seenMu.RUnlock()

	r.seenMu.Lock()
	defer r.seenMu.Unlock()
	if r.seenDomain[domain] {
		return false
	}
	r.seenDomain[domain] = true
	return true
}

func (r *Runner) notifyDomain(domain string) {
	if r.notifier == nil {
		return
	}
	go r.notifier.Notify(domain)
}

func (r *Runner) hasRootFilter() bool {
	r.rootMu.RLock()
	defer r.rootMu.RUnlock()
	return len(r.rootDomains) > 0
}

func (r *Runner) matchingRoot(domain string) (string, bool) {
	r.rootMu.RLock()
	defer r.rootMu.RUnlock()
	if len(r.rootDomains) == 0 {
		return "", true
	}

	if r.rootDomains[domain] {
		return domain, true
	}

	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[i:], ".")
		if r.rootDomains[parent] {
			return parent, true
		}
	}
	return "", false
}

func (r *Runner) loadRootDomains() error {
	if r.options.RootList == "" {
		return nil
	}

	file, err := os.Open(r.options.RootList)
	if err != nil {
		return fmt.Errorf("failed to open root domains file: %w", err)
	}
	defer file.Close()

	newRoots := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		newRoots[strings.ToLower(line)] = true
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading root domains: %w", err)
	}

	r.rootMu.Lock()
	r.rootDomains = newRoots
	r.rootMu.Unlock()

	log.Printf("[*] Loaded %d root domains", len(newRoots))
	return nil
}

func (r *Runner) setupWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to initialize watcher: %w", err)
	}
	if err := watcher.Add(r.options.RootList); err != nil {
		return fmt.Errorf("failed to watch %s: %w", r.options.RootList, err)
	}
	r.watcher = watcher
	r.restartChan = make(chan struct{}, 1)
	go r.watchRootFile()
	return nil
}

func (r *Runner) watchRootFile() {
	for {
		select {
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				if event.Op&fsnotify.Rename != 0 {
					_ = r.watcher.Add(r.options.RootList)
				}
				if err := r.loadRootDomains(); err != nil {
					log.Printf("Error reloading root domains: %v", err)
					continue
				}
				select {
				case r.restartChan <- struct{}{}:
				default:
				}
			}
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func (r *Runner) restartChannel() <-chan struct{} {
	return r.restartChan
}

func (r *Runner) closeWatcher() {
	if r.watcher != nil {
		_ = r.watcher.Close()
	}
}
