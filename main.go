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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

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
)

// cliOptions holds the parsed CLI flag values and the resolved dispatch mode.
type cliOptions struct {
	configPath  string
	mode        cliMode
	backupCount int
	applyModel  string
	agentsCSV   string
}

// parseFlags parses command-line arguments into cliOptions. It returns the
// parsed options and an exit code: 0 for success, 2 for flag/usage errors
// (the standard convention for CLI usage errors).
//
// Output from the flag package is suppressed (io.Discard) so that parseFlags
// is a pure function; run() handles any user-facing error messaging.
//
// Mode precedence (REQ-CMD-001): list-models > list-agents > apply-model > TUI.
func parseFlags(args []string) (cliOptions, int) {
	fs := flag.NewFlagSet("opencode-model-selector", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // pure function — run() handles user-facing output

	var opts cliOptions
	var listModels, listAgents bool

	fs.StringVar(&opts.configPath, "config", "", "Override config file path")
	fs.BoolVar(&listModels, "list-models", false, "List available models grouped by provider")
	fs.BoolVar(&listAgents, "list-agents", false, "List agents with current field values")
	fs.IntVar(&opts.backupCount, "backup-count", 5, "Number of backups to retain (0 to disable)")
	fs.StringVar(&opts.applyModel, "apply-model", "", "Apply model to agents (requires --agents)")
	fs.StringVar(&opts.agentsCSV, "agents", "", "Target agents: 'all' or comma-separated names")

	if err := fs.Parse(args); err != nil {
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
	opts, exitCode := parseFlags(args)
	if exitCode != 0 {
		return exitCode
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
		if err := runListAgents(cfg); err != nil {
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
		if err := runTUI(cfg, grouped, opts.backupCount); err != nil {
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

// runListAgents prints all agents with their current field values to stdout.
//
// Spec: REQ-CMD-003 — implemented in G3-T2.
func runListAgents(cfg *config.Config) error {
	return formatAgents(os.Stdout, cfg)
}

// runTUI launches the interactive Bubbletea terminal UI in alt-screen mode.
//
// This is a thin wrapper over tea.NewProgram — it constructs the TUI model
// via tui.NewModel and runs the program. The function cannot be unit-tested
// in CI because program.Run() takes over the terminal; manual testing only.
//
// Spec: REQ-CMD-005, REQ-TUI-001
func runTUI(cfg *config.Config, grouped map[string][]opencode.Model, backupCount int) error {
	model := tui.NewModel(cfg, grouped, backupCount)
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err := program.Run()
	return err
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

// agentFieldDef defines one of the 6 editable agent fields and how to render it.
type agentFieldDef struct {
	name string
	kind fieldKind
}

// fieldKind controls how a field value extracted from config is formatted.
type fieldKind int

const (
	fieldString fieldKind = iota
	fieldFloat
	fieldInt
	fieldBool
)

// agentFields is the ordered list of fields shown for every agent.
var agentFields = []agentFieldDef{
	{"model", fieldString},
	{"temperature", fieldFloat},
	{"top_p", fieldFloat},
	{"color", fieldString},
	{"steps", fieldInt},
	{"disable", fieldBool},
}

// formatFieldValue extracts a field from config and formats it according to kind.
// Boolean fields default to "false" when absent; all other kinds show "(none)".
func formatFieldValue(cfg *config.Config, agentName, fieldName string, kind fieldKind) string {
	val, ok := cfg.GetAgentField(agentName, fieldName)
	if !ok || val == nil {
		if kind == fieldBool {
			return "false"
		}
		return "(none)"
	}

	switch kind {
	case fieldString:
		s, ok := val.(string)
		if !ok {
			return "(none)"
		}
		return s
	case fieldFloat:
		f, ok := val.(float64)
		if !ok {
			return "(none)"
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	case fieldInt:
		f, ok := val.(float64)
		if !ok {
			return "(none)"
		}
		return strconv.FormatInt(int64(f), 10)
	case fieldBool:
		b, ok := val.(bool)
		if !ok {
			return "false"
		}
		if b {
			return "true"
		}
		return "false"
	}
	return "(none)"
}

// writeAgentBlock writes one agent's name and 6 fields to w.
func writeAgentBlock(w io.Writer, cfg *config.Config, name string) error {
	// Agent name with optional [H] marker for hidden agents.
	if cfg.IsAgentHidden(name) {
		if _, err := fmt.Fprintf(w, "  %s [H]\n", name); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "  %s\n", name); err != nil {
			return err
		}
	}

	// Six editable fields, label column padded to 12 runes + space.
	for _, f := range agentFields {
		val := formatFieldValue(cfg, name, f.name, f.kind)
		marker := ""
		if f.name == "disable" && val == "true" {
			marker = "                   [DISABLED]"
		}
		if _, err := fmt.Fprintf(w, "    %-12s %s%s\n", f.name+":", val, marker); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil { // blank line between agents
		return err
	}
	return nil
}

// formatAgents writes the agent listing to w.
//
// Output structure (REQ-CMD-003):
//   - "OpenCode Agents" header
//   - "Global Default Model: <model>" (or "(none)")
//   - "── Primary Agents ──..." with each primary agent and 6 fields
//   - "── Subagents ──..." with each subagent and 6 fields
//   - System agents (compactación, title, summary) are excluded
//   - Disabled agents marked with [DISABLED]
//   - Hidden agents marked with [H]
func formatAgents(w io.Writer, cfg *config.Config) error {
	// Header
	if _, err := fmt.Fprintln(w, "OpenCode Agents"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Global default model
	globalModel, ok := cfg.GetGlobalModel()
	if ok {
		if _, err := fmt.Fprintf(w, "Global Default Model: %s\n", globalModel); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, "Global Default Model: (none)"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Get agents grouped by mode
	primary, subagents, _ := cfg.GetAgents()

	// Sort alphabetically for deterministic output
	sort.Strings(primary)
	sort.Strings(subagents)

	// Primary agents section
	if _, err := fmt.Fprintln(w, separatorLine("Primary Agents")); err != nil {
		return err
	}
	for _, name := range primary {
		if err := writeAgentBlock(w, cfg, name); err != nil {
			return err
		}
	}

	// Subagents section
	if _, err := fmt.Fprintln(w, separatorLine("Subagents")); err != nil {
		return err
	}
	for _, name := range subagents {
		if err := writeAgentBlock(w, cfg, name); err != nil {
			return err
		}
	}

	return nil
}
