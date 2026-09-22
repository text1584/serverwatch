package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	target := flag.String("url", "", "Apache server-status URL")
	interval := flag.Duration("interval", 30*time.Second, "poll interval, e.g. 10s, 30s, 1m")
	output := flag.String("output", "output", "output directory")
	includeStatic := flag.Bool("include-static", false, "include static assets such as .js/.css/.png")
	timeout := flag.Duration("timeout", 15*time.Second, "HTTP request timeout")
	flag.Parse()

	if *target == "" {
		fmt.Println("Usage:")
		fmt.Println("  serverwatch -url https://target.example/server-status")
		fmt.Println("")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(2)
	}
	if *interval <= 0 {
		fmt.Println("interval must be > 0")
		os.Exit(2)
	}

	if err := os.MkdirAll(filepath.Clean(*output), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "output: %v\n", err)
		os.Exit(1)
	}

	store, err := NewStore(*output, *target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "state: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: *timeout}

	fmt.Print(`
  ____                          __        ___        __  __
 / ___|  ___ _ ____   _____ _ _\ \      / / |_ _ _ _\ \/ /
 \___ \ / _ \ '__\ \ / / _ \ '__\ \ /\ / /| __| '__|\  /
  ___) |  __/ |   \ V /  __/ |     \ V  / | |_| |   /  \
 |____/ \___|_|    \_/ \___|_|      \_/   \__|_|  /_/\_\
`)
	fmt.Println("Apache server-status monitor")
	fmt.Println("Target :", *target)
	fmt.Println("Every  :", *interval)
	fmt.Println("Output :", *output)
	fmt.Println("Press CTRL+C to stop.")
	fmt.Println()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	monitor := &Monitor{
		URL:        *target,
		Interval:   *interval,
		Client:     client,
		Store:      store,
		ShowStatic: *includeStatic,
	}

	if err := monitor.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "monitor: %v\n", err)
		os.Exit(1)
	}

	_ = store.Export()
	fmt.Println("State flushed. Exiting.")
}
