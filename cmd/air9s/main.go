package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AymanZahran/air9s/internal/act"
	"github.com/AymanZahran/air9s/internal/config"
	"github.com/AymanZahran/air9s/internal/index"
	"github.com/AymanZahran/air9s/internal/model"
	"github.com/AymanZahran/air9s/internal/query"
	"github.com/AymanZahran/air9s/internal/store"
	"github.com/AymanZahran/air9s/internal/tui"
)

const version = "0.2.7"

func main() {
	if len(os.Args) < 2 {
		os.Exit(runTUI())
	}
	switch os.Args[1] {
	case "index":
		os.Exit(cmdIndex(os.Args[2:]))
	case "stats":
		os.Exit(cmdStats(os.Args[2:]))
	case "search":
		os.Exit(cmdSearch(os.Args[2:]))
	case "show":
		os.Exit(cmdShow(os.Args[2:]))
	case "resume":
		os.Exit(cmdResume(os.Args[2:]))
	case "delete":
		os.Exit(cmdDelete(os.Args[2:]))
	case "info":
		os.Exit(cmdInfo())
	case "version", "--version", "-v":
		fmt.Println("air9s", version)
	case "help", "--help", "-h":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `air9s %s — find and resume local AI coding sessions

Usage:
  air9s                         open the session list
  air9s index                   scan agent session stores
  air9s stats [--json]          counts by agent
  air9s search [query] [--json] [--limit N]
  air9s show <id> [--json]
  air9s resume <id> [--yolo] [--print]
  air9s delete <id> [--yes]
  air9s info                    config directory, index path, skin, plugins

Queries can mix free text with agent:, dir:, branch:, model:,
date:<7d, date:>30d, date:YYYY-MM-DD, and sort:recent|oldest|messages|title.
Free text and agent:, dir:, branch:, and model: match substrings. A leading ~ expands.

The index is cached under $AIR9S_CACHE_DIR, $XDG_CACHE_HOME/air9s, or ~/.cache/air9s.
Config is $AIR9S_CONFIG_DIR, $XDG_CONFIG_HOME/air9s, or ~/.config/air9s.
`, version)
}

func openStore() (*store.Store, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return nil, err
	}
	return store.Open(path)
}

func runTUI() int {
	if !isTerm(os.Stdin) {
		fmt.Fprintln(os.Stderr, "air9s: no terminal; try 'air9s search' or 'air9s stats'")
		return 1
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "air9s:", err)
		return 1
	}
	defer st.Close()
	fmt.Fprintln(os.Stderr, "air9s: indexing sessions")
	_, warnings, err := index.Rebuild(st)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "air9s:", w)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "air9s:", err)
		return 1
	}
	cmd, err := tui.Run(st)
	if err != nil {
		fmt.Fprintln(os.Stderr, "air9s:", err)
		return 1
	}
	if cmd == nil {
		return 0
	}
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "air9s:", err)
		return 1
	}
	return 0
}

func cmdInfo() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "air9s:", err)
		return 1
	}
	cache, err := store.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "air9s:", err)
		return 1
	}
	mouse := "on"
	if !cfg.Mouse() {
		mouse = "off"
	}
	readOnly := "no"
	if cfg.Body.ReadOnly {
		readOnly = "yes"
	}
	fmt.Printf("config    %s\n", cfg.Path)
	fmt.Printf("cache     %s\n", cache)
	fmt.Printf("skin      %s\n", cfg.SkinName)
	fmt.Printf("plugins   %d\n", len(cfg.Plugins))
	fmt.Printf("mouse     %s\n", mouse)
	fmt.Printf("readOnly  %s\n", readOnly)
	fmt.Printf("view      %s\n", cfg.Body.DefaultView)
	for _, w := range cfg.Warnings {
		fmt.Fprintln(os.Stderr, "air9s:", w)
	}
	return 0
}

func cmdIndex(args []string) int {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print stats as JSON")
	fs.SetOutput(os.Stderr)
	if _, err := parseArgs(fs, args); err != nil {
		return 2
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer st.Close()
	stats, warnings, err := index.Rebuild(st)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *asJSON {
		return printJSON(stats)
	}
	printStats(stats)
	return 0
}

func cmdStats(args []string) int {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON")
	if _, err := parseArgs(fs, args); err != nil {
		return 2
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer st.Close()
	stats, err := st.Stats()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *asJSON {
		return printJSON(stats)
	}
	printStats(stats)
	return 0
}

func printStats(st store.Stats) {
	fmt.Printf("%d sessions, %d messages\n", st.Sessions, st.Messages)
	for _, a := range st.Agents {
		fmt.Printf("  %s %-10s %5d sessions  %7d messages\n", tui.Icon(a.Agent), a.Agent, a.Sessions, a.Messages)
	}
}

func cmdSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON")
	limit := fs.Int("limit", 50, "maximum rows")
	fs.SetOutput(os.Stderr)
	rest, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer st.Close()
	rows, err := st.Search(query.Parse(strings.Join(rest, " ")), *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *asJSON {
		return printJSON(rows)
	}
	if len(rows) == 0 {
		fmt.Println("no sessions")
		return 0
	}
	for _, s := range rows {
		branch := s.Branch
		if branch == "" {
			branch = "-"
		}
		fmt.Printf("%s %-8s %-18s %-10s %5d  %-48s  %s\n", tui.Icon(s.Agent), s.Agent, clip(shortHome(s.CWD), 18), branch, s.Messages, clip(s.Title, 48), s.ID)
	}
	return 0
}

func cmdShow(args []string) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON")
	rest, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "usage: air9s show <id>")
		return 2
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer st.Close()
	sess, err := st.Resolve(rest[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *asJSON {
		return printJSON(sess)
	}
	fmt.Printf("%s\n%s %s  %s\n", sess.Title, tui.Icon(sess.Agent), sess.Agent, sess.ID)
	fmt.Printf("directory  %s\n", emptyDash(sess.CWD))
	fmt.Printf("branch     %s\n", emptyDash(sess.Branch))
	fmt.Printf("model      %s\n", emptyDash(sess.Model))
	fmt.Printf("updated    %s\n", formatTime(sess.Updated))
	fmt.Printf("messages   %d\n", sess.Messages)
	for _, line := range tui.UsageLines(sess.Usage) {
		fmt.Println(line)
	}
	if sess.CanDelete {
		fmt.Printf("delete     yes (%s)\n", sess.DeleteMode)
	} else {
		fmt.Printf("delete     no\n%s\n", sess.DeleteReason)
	}
	if sess.Summary != "" {
		fmt.Printf("\n%s\n", sess.Summary)
	}
	for _, sn := range sess.Snippets {
		fmt.Printf("\n%s\n%s\n", strings.ToUpper(sn.Role), sn.Body)
	}
	return 0
}

func cmdResume(args []string) int {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	yolo := fs.Bool("yolo", false, "pass the agent's auto-approve flag when one exists")
	printOnly := fs.Bool("print", false, "print the command instead of running it")
	rest, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "usage: air9s resume <id>")
		return 2
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer st.Close()
	sess, err := st.Resolve(rest[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cmd, err := act.Plan(sess, *yolo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *printOnly {
		fmt.Println(cmd.String())
		return 0
	}
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdDelete(args []string) int {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "delete without a confirmation prompt")
	rest, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "usage: air9s delete <id> [--yes]")
		return 2
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer st.Close()
	sess, err := st.Resolve(rest[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !sess.CanDelete {
		if sess.DeleteReason != "" {
			fmt.Fprintln(os.Stderr, sess.DeleteReason)
		} else {
			fmt.Fprintf(os.Stderr, "deletion is disabled for %s\n", sess.Agent)
		}
		return 1
	}
	if !*yes && !confirm(sess) {
		fmt.Fprintln(os.Stderr, "not deleted")
		return 1
	}
	if err := act.Delete(sess); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := st.Forget(sess.ID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("deleted %s\n", sess.ID)
	return 0
}

// parseArgs accepts flags before or after the session id and query text.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	valued := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		bf, ok := f.Value.(interface{ IsBoolFlag() bool })
		if !ok || !bf.IsBoolFlag() {
			valued[f.Name] = true
		}
	})
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			rest = append(rest, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		flags = append(flags, a)
		if valued[name] && !strings.Contains(a, "=") {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag needs an argument: %s", a)
			}
			i++
			flags = append(flags, args[i])
		}
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return rest, nil
}

func confirm(s model.Session) bool {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		fmt.Fprintln(os.Stderr, "refusing to delete without --yes")
		return false
	}
	fmt.Printf("Delete %s (%s)? Type %s to confirm: ", s.Title, s.ID, s.NativeID)
	var line string
	fmt.Scanln(&line)
	return strings.TrimSpace(line) == s.NativeID
}

func printJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func isTerm(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format(time.RFC3339)
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func shortHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || p == "" {
		return emptyDash(p)
	}
	if p == home || strings.HasPrefix(p, home+string(os.PathSeparator)) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
