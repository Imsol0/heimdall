package runner

import (
	"flag"
	"fmt"
)

type Options struct {
	Verbose        bool
	RootList       string
	Debug          bool
	JsonOutput     bool
	WatchFile      bool
	OutputDir      string
	DiscordWebhook string
}

func ParseOptions() (*Options, error) {
	options := &Options{}

	flag.StringVar(&options.RootList, "r", "", "Path to the list of root domains to filter against")
	flag.BoolVar(&options.WatchFile, "f", false, "Monitor the root domain file for updates and restart the scan. requires the -r flag")
	flag.BoolVar(&options.Verbose, "v", false, "Output go logs (500/429 errors) to command line")
	flag.BoolVar(&options.Debug, "debug", false, "Debug CT logs to see if you are keeping up")
	flag.BoolVar(&options.JsonOutput, "j", false, "JSONL output cert info")
	flag.StringVar(&options.OutputDir, "o", "", "Directory to store output files (one per hostname, requires -r flag)")
	flag.StringVar(&options.DiscordWebhook, "discord-webhook", "", "Discord webhook URL for notifications")
	flag.Parse()

	if options.OutputDir != "" && options.RootList == "" {
		return nil, fmt.Errorf("the -o flag requires the -r flag to be set")
	}

	return options, nil
}
