package cmd

import (
	"fmt"
	"os"

	"github.com/AymanZahran/ai9s/internal/config"
)

// version, commit, and date are set with -ldflags -X at build time.
// A build without those flags prints dev, the same way k9s does.
var (
	version = "dev"
	commit  = "dev"
	date    = "dev"
)

// Execute runs the ai9s command and exits.
func Execute() {
	os.Exit(dispatch())
}

func dispatch() int {
	if len(os.Args) < 2 {
		usage()
		return 2
	}
	switch os.Args[1] {
	case "info":
		return cmdInfo()
	case "version", "--version", "-v":
		fmt.Println("ai9s", version)
		return 0
	case "help", "--help", "-h":
		usage()
		return 0
	default:
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `ai9s %s — find and resume local AI coding sessions

Usage:
  ai9s version                 print the version
  ai9s info                    config directory and build
  ai9s help

`, version)
}

func cmdInfo() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ai9s:", err)
		return 1
	}
	fmt.Printf("version   %s\n", version)
	fmt.Printf("commit    %s\n", commit)
	fmt.Printf("built     %s\n", date)
	fmt.Printf("config    %s\n", cfg.Path)
	fmt.Printf("skin      %s\n", cfg.SkinName)
	fmt.Printf("plugins   %d\n", len(cfg.Plugins))
	for _, w := range cfg.Warnings {
		fmt.Fprintln(os.Stderr, "ai9s:", w)
	}
	return 0
}
