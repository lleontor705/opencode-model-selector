// Package main is the CLI entry point for opencode-model-selector. It parses
// flags using the stdlib flag package and dispatches to the appropriate mode
// (interactive TUI, list-models, list-agents, or apply-model).
//
// No business logic lives here — this file is pure flag parsing and dispatch
// routing. The actual output functions (runListModels, runListAgents, runTUI)
// are implemented in subsequent tasks (G3-T2, G3-T3).
//
// Spec: REQ-CMD-001
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/opencode-model-selector/internal/agentcatalog"
	"github.com/lleontor705/opencode-model-selector/internal/appname"
	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
	"github.com/lleontor705/opencode-model-selector/internal/tui"
)

// cliMode enumerates the dispatch modes selected by CLI flags.
type cliMode string

const (
	modeTUI        cliMode = "tui"
	modeListModels cliMode = "list-models"
	modeListAgents cliMode = "list-agents"
	modeApplyModel cliMode = "apply-model"
	listAgentsHelp         = "List the runtime agent catalog"
)

// cliOptions holds the parsed CLI flag values and the resolved dispatch mode.
type cliOptions struct {
	configPath  string
	mode        cliMode
	backupCount int
	applyModel  string
	agentsCSV   string
	showVersion bool
}

// version is the stamped version string, injected at link time via
// -ldflags "-X main.version=...". It is empty for local `go build`/`go install`
// runs; printVersion renders the "(dev)" fallback in that case.
var version string

// printVersion writes the version (or "(dev)" when unstamped) to w, followed by
// a newline. It performs no I/O beyond the write and touches no config, so it
// is safe to call as the very first action of run().
func printVersion(w io.Writer) {
	if version == "" {
		fmt.Fprintln(w, "(dev)")
		return
	}
	fmt.Fprintln(w, version)
}

// printUsage writes the program usage banner to w. The program name is the
// canonical "ocs" (appname.Name) and every registered flag — including
// --version — is listed. Writing goes to w directly so the banner is visible
// even though parseFlags suppresses the flag package's own output via
// SetOutput(io.Discard).
func printUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintf(w, "Usage: %s [flags]\n\n", appname.Name)
	fmt.Fprintln(w, "Flags:")
	fs.SetOutput(w)
	fs.PrintDefaults()
}

// parseFlags parses command-line arguments into cliOptions. It returns the
// parsed options and an exit code: 0 for success or -h/--help, 2 for flag/usage
// errors (the standard convention for CLI usage errors).
//
// The flag package's own error output is suppressed (io.Discard) so parseFlags
// stays pure on the success path; a custom Usage banner (program name "ocs",
// listing --version) is shown on -h/--help (stdout, exit 0) and on flag errors
// (stderr, exit 2) via printUsage.
//
// Mode precedence (REQ-CMD-001): list-models > list-agents > apply-model > TUI.
func parseFlags(args []string) (cliOptions, int) {
	fs := flag.NewFlagSet(appname.Name, flag.ContinueOnError)
	fs.SetOutput(io.Discard) // suppress internal flag-package error chatter

	var opts cliOptions
	var listModels, listAgents bool

	fs.StringVar(&opts.configPath, "config", "", "Override config file path")
	fs.BoolVar(&listModels, "list-models", false, "List available models grouped by provider")
	fs.BoolVar(&listAgents, "list-agents", false, listAgentsHelp)
	fs.IntVar(&opts.backupCount, "backup-count", 5, "Number of backups to retain (0 to disable)")
	fs.StringVar(&opts.applyModel, "apply-model", "", "Apply model to agents (requires --agents)")
	fs.StringVar(&opts.agentsCSV, "agents", "", "Target agents: 'all' or comma-separated names")
	fs.BoolVar(&opts.showVersion, "version", false, "Print version and exit")

	// -h/--help: print the banner to stdout and signal exit 0. Genuine flag
	// errors print the banner to stderr and exit 2. The Usage callback inspects
	// the parse result via a closure variable to pick the right stream.
	var parseErr error
	fs.Usage = func() {
		if errors.Is(parseErr, flag.ErrHelp) {
			printUsage(os.Stdout, fs)
		} else {
			printUsage(os.Stderr, fs)
		}
	}

	parseErr = fs.Parse(args)
	if parseErr != nil {
		if errors.Is(parseErr, flag.ErrHelp) {
			return opts, 0 // help is not an error
		}
		return opts, 2
	}

	if opts.applyModel != "" && opts.agentsCSV == "" {
		return opts, 2
	}
	if opts.agentsCSV != "" && opts.applyModel == "" {
		return opts, 2
	}

	switch {
	case listModels:
		opts.mode = modeListModels
	case listAgents:
		opts.mode = modeListAgents
	case opts.applyModel != "":
		opts.mode = modeApplyModel
	default:
		opts.mode = modeTUI
	}

	return opts, 0
}

// run is the testable entry point. It parses flags, resolves the config path,
// loads the config, and dispatches to the appropriate mode handler.
//
// Exit codes:
//   - 0: success
//   - 1: runtime error (opencode not found, config not found, parse error, etc.)
//   - 2: flag/usage error
//
// Spec: REQ-CMD-001, REQ-CMD-004
func run(args []string) int {
	return runWithAgentDiscovery(args, productionAgentDiscovery)
}

// agentDiscoverer is the narrow seam used by CLI tests to avoid starting a
// real OpenCode runtime. Production constructs agentcatalog.Discovery above.
type agentDiscoverer interface {
	Discover(context.Context, string) agentcatalog.Catalog
}

type agentDiscoveryFactory func(*config.Config) agentDiscoverer

func productionAgentDiscovery(cfg *config.Config) agentDiscoverer {
	return agentcatalog.Discovery{Runtime: opencode.NewRuntimeAgentProbe(), Static: cfg}
}

func loadAgentCatalog(cfg *config.Config, newDiscovery agentDiscoveryFactory) (agentcatalog.Catalog, error) {
	if newDiscovery == nil {
		return agentcatalog.Catalog{}, errors.New("agent discovery is unavailable")
	}
	directory, err := os.Getwd()
	if err != nil {
		return agentcatalog.Catalog{}, fmt.Errorf("resolving runtime directory: %w", err)
	}
	catalog := newDiscovery(cfg).Discover(context.Background(), directory)
	if catalog.Degraded {
		fmt.Fprintln(os.Stderr, "Warning: runtime agent discovery failed; using static catalog")
	}
	return catalog, nil
}

func runWithAgentDiscovery(args []string, newDiscovery agentDiscoveryFactory) int {
	opts, exitCode := parseFlags(args)
	if exitCode != 0 {
		return exitCode
	}

	// --version short-circuits BEFORE any config resolution or side effect
	// (VERSION-003). It must succeed even when no valid config exists.
	if opts.showVersion {
		printVersion(os.Stdout)
		return 0
	}

	// Resolve config path (flag override or default).
	configPath, err := config.GetConfigPath(opts.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving config path: %v\n", err)
		return 1
	}

	// Load config — required by all modes.
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			// Spec: REQ-CMD-004 — "Config not found at {path}"
			fmt.Fprintf(os.Stderr, "Config not found at %s\n", configPath)
		} else {
			// Spec: REQ-CMD-004 — "Error loading config: {parse error}"
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		}
		return 1
	}

	// Dispatch based on mode.
	switch opts.mode {
	case modeListModels:
		// list-models requires the opencode CLI for model retrieval.
		// Spec: REQ-CMD-004 — explicit Detect() gives the user-friendly
		// install message before the deeper GetModels error.
		if _, err := opencode.Detect(); err != nil {
			fmt.Fprintf(os.Stderr, "opencode CLI not found. Install it: https://opencode.ai\n")
			return 1
		}
		models, err := opencode.GetModels()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting models: %v\n", err)
			return 1
		}
		if err := runListModels(cfg, models); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case modeListAgents:
		catalog, err := loadAgentCatalog(cfg, newDiscovery)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if err := runListAgents(catalog); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case modeApplyModel:
		if err := runApplyModel(cfg, opts.applyModel, opts.agentsCSV, opts.backupCount); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case modeTUI:
		// TUI requires both config and available models.
		// Spec: REQ-CMD-004 — explicit Detect() gives the user-friendly
		// install message before the deeper GetModels error.
		if _, err := opencode.Detect(); err != nil {
			fmt.Fprintf(os.Stderr, "opencode CLI not found. Install it: https://opencode.ai\n")
			return 1
		}
		models, err := opencode.GetModels()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting models: %v\n", err)
			return 1
		}
		grouped := opencode.GroupByProvider(models)
		if err := runTUIWithDependencies(cfg, grouped, opts.backupCount, newDiscovery, newTUIModel, runTUIProgram); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}

	return 0
}

// main is the process entry point. It delegates to run() and exits with the
// returned code.
func main() {
	os.Exit(run(os.Args[1:]))
}

// --- Stubs (implemented in G3-T2 and G3-T3) ---

// runApplyModel executes the CLI bulk model apply. It detects opencode, fetches
// available models, and delegates to applyModelWithModels for the testable
// business logic.
//
// Exit codes (returned as error-nil/non-nil by this function):
//   - nil: success
//   - non-nil: runtime error (opencode missing, invalid model, save failure)
//
// Spec: REQ-CLI-003..008
func runApplyModel(cfg *config.Config, applyModel, agentsCSV string, backupCount int) error {
	if _, err := opencode.Detect(); err != nil {
		return fmt.Errorf("opencode CLI not found. Install it: https://opencode.ai")
	}
	models, err := opencode.GetModels()
	if err != nil {
		return fmt.Errorf("getting models: %w", err)
	}
	return applyModelWithModels(cfg, applyModel, agentsCSV, backupCount, models)
}

// applyModelWithModels is the testable core of runApplyModel. It accepts the
// available models as a parameter so tests can exercise it without the opencode
// binary.
//
// Flow:
//  1. Parse agentsCSV: "all" → nil names; else split+trim CSV
//  2. config.ApplyModelToAgents (validates model, applies, collects skips)
//  3. If empty result set → "0 agents updated", return nil (no backup, no save)
//  4. CreateBackup → cfg.Save() → CleanOldBackups (if backupCount > 0)
//  5. Print applied/skipped summary to stdout
//
// Spec: REQ-CLI-003..008
func applyModelWithModels(cfg *config.Config, applyModel, agentsCSV string, backupCount int, models []opencode.Model) error {
	var names []string
	if agentsCSV != "all" {
		for _, n := range strings.Split(agentsCSV, ",") {
			n = strings.TrimSpace(n)
			if n != "" {
				names = append(names, n)
			}
		}
	}

	applied, skipped, err := config.ApplyModelToAgents(cfg, applyModel, names, models)
	if err != nil {
		return err
	}

	if len(applied) == 0 && len(skipped) == 0 {
		fmt.Println("0 agents updated")
		return nil
	}

	if backupCount > 0 {
		if _, err := config.CreateBackup(cfg.Path()); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}
	if backupCount > 0 {
		_ = config.CleanOldBackups(cfg.Path(), backupCount)
	}

	fmt.Printf("Model %s applied to %d agent%s\n", applyModel, len(applied), pluralCLI(len(applied)))
	for _, name := range applied {
		fmt.Printf("  ✓ %s\n", name)
	}
	if len(skipped) > 0 {
		fmt.Printf("Skipped %d agent%s (disabled or already up-to-date)\n", len(skipped), pluralCLI(len(skipped)))
		for _, name := range skipped {
			fmt.Printf("  - %s\n", name)
		}
	}
	return nil
}

// pluralCLI returns "" for singular (1) or "s" for plural.
func pluralCLI(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// runListModels prints all available models grouped by provider to stdout.
//
// Spec: REQ-CMD-002 — implemented in G3-T2.
func runListModels(cfg *config.Config, models []opencode.Model) error {
	return formatModels(os.Stdout, models)
}

// runListAgents prints the authoritative unified catalog to stdout.
func runListAgents(catalog agentcatalog.Catalog) error {
	return formatAgents(os.Stdout, catalog)
}

// runTUI launches the interactive Bubbletea terminal UI with authoritative
// runtime discovery and static degraded fallback.
//
// Spec: REQ-CMD-005, REQ-TUI-001
func runTUI(cfg *config.Config, grouped map[string][]opencode.Model, backupCount int) error {
	return runTUIWithDependencies(cfg, grouped, backupCount, productionAgentDiscovery, newTUIModel, runTUIProgram)
}

type tuiModelFactory func(*config.Config, map[string][]opencode.Model, int, agentcatalog.Catalog) tea.Model
type tuiRunner func(tea.Model) error

func newTUIModel(cfg *config.Config, grouped map[string][]opencode.Model, backupCount int, catalog agentcatalog.Catalog) tea.Model {
	return tui.NewModelWithCatalog(cfg, grouped, backupCount, catalog)
}

func runTUIProgram(model tea.Model) error {
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func runTUIWithDependencies(cfg *config.Config, grouped map[string][]opencode.Model, backupCount int, newDiscovery agentDiscoveryFactory, newModel tuiModelFactory, runProgram tuiRunner) error {
	catalog, err := loadAgentCatalog(cfg, newDiscovery)
	if err != nil {
		return err
	}
	return runProgram(newModel(cfg, grouped, backupCount, catalog))
}

// --- Model and Agent Formatting (REQ-CMD-002, REQ-CMD-003) ---

// sectionLineWidth is the visual width of section separator lines (in runes).
const sectionLineWidth = 50

// separatorLine builds a "── label ────...──" line padded to sectionLineWidth
// runes. This is used for both model provider headers and agent section headers.
func separatorLine(label string) string {
	prefix := "── " + label + " "
	width := utf8.RuneCountInString(prefix)
	padding := sectionLineWidth - width
	if padding < 1 {
		padding = 1
	}
	return prefix + strings.Repeat("─", padding)
}

// formatModels writes the grouped model listing to w.
//
// Output structure (REQ-CMD-002):
//   - "Available Models (N total)" header
//   - Per-provider sections ("── provider/ (count) ──...") with model IDs
//   - "Total: N models across M providers" footer
//   - Empty input produces "No models available"
func formatModels(w io.Writer, models []opencode.Model) error {
	if len(models) == 0 {
		if _, err := fmt.Fprintln(w, "No models available"); err != nil {
			return err
		}
		return nil
	}

	grouped := opencode.GroupByProvider(models)

	// Sort provider names alphabetically for deterministic output.
	providers := make([]string, 0, len(grouped))
	for p := range grouped {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	// Header
	if _, err := fmt.Fprintf(w, "Available Models (%d total)\n\n", len(models)); err != nil {
		return err
	}

	// Provider sections
	for _, p := range providers {
		pModels := grouped[p]
		if _, err := fmt.Fprintln(w, separatorLine(fmt.Sprintf("%s/ (%d)", p, len(pModels)))); err != nil {
			return err
		}
		for _, m := range pModels {
			if _, err := fmt.Fprintf(w, "  %s\n", m.ID); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil { // blank line between sections
			return err
		}
	}

	// Footer
	if _, err := fmt.Fprintf(w, "Total: %d models across %d providers\n", len(models), len(providers)); err != nil {
		return err
	}

	return nil
}

// formatAgents writes a tab-separated, script-friendly catalog containing only
// section, name, mode, model, and status. Bucket ordering comes from Catalog.
func formatAgents(w io.Writer, catalog agentcatalog.Catalog) error {
	if _, err := fmt.Fprintln(w, "section\tname\tmode\tmodel\tstatus"); err != nil {
		return err
	}
	sections := []struct {
		name    string
		records []agentcatalog.AgentRecord
	}{
		{name: "Primary", records: catalog.Buckets.Primary},
		{name: "Subagent", records: catalog.Buckets.Subagent},
		{name: "All", records: catalog.Buckets.All},
	}
	for _, section := range sections {
		for _, record := range section.records {
			status := "custom"
			switch {
			case catalog.Degraded:
				status = "degraded"
			case record.Native:
				status = "native"
			case record.Hidden:
				status = "hidden(custom)"
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", section.name, record.Name, record.Role, record.Model, status); err != nil {
				return err
			}
		}
	}
	return nil
}
