// Package tui implements the Bubbletea terminal UI for the opencode model
// selector using the Model-View-Update architecture.
//
// This file contains the root Model struct, the appState state machine, and
// the global Update() dispatcher. Per-screen view and key handling live in
// sibling files (agent_list.go, agent_detail.go, model_select.go,
// field_input.go, save_confirm.go) implemented in subsequent tasks. Until
// those land, View() returns placeholder strings so the dispatcher is fully
// exercised by tests.
//
// Spec coverage: REQ-TUI-001 (initialization), REQ-TUI-008 (transitions).
package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// appState enumerates the five screens of the TUI state machine (design #606,
// Section 2 Decision 3). Ordering matters only for iota stability; do NOT
// re-order existing entries.
type appState int

const (
	// ScreenAgentList is the entry screen: lists primary agents, subagents,
	// and the global default model (REQ-TUI-002, REQ-TUI-003).
	ScreenAgentList appState = iota
	// ScreenAgentDetail shows the 6 editable fields for a single agent
	// (REQ-TUI-004).
	ScreenAgentDetail
	// ScreenModelSelection shows all available models grouped by provider
	// with a fuzzy filter (REQ-TUI-005).
	ScreenModelSelection
	// ScreenFieldInput captures free-form text for non-model fields
	// (temperature, top_p, color, steps) with per-field validation
	// (REQ-TUI-006).
	ScreenFieldInput
	// ScreenSaveConfirm shows a summary of pending changes and triggers the
	// atomic save flow (REQ-TUI-007).
	ScreenSaveConfirm
)

// editableFieldSchema is the fixed list of fields exposed on the Agent Detail
// screen. Order matches the design's transition table (Section 7) and the
// validation table (Section 7.1).
var editableFieldSchema = []string{
	"model", "temperature", "top_p", "color", "steps", "disable",
}

// Change records a single in-memory mutation that will be persisted on save.
// Target is "global" for the global default model or the agent name for
// per-agent edits; Field is the config key (model, temperature, ...). OldVal
// and NewVal carry the raw interface{} values from the config layer so the
// save-confirm screen can render a human-readable diff.
type Change struct {
	Target string
	Field  string
	OldVal interface{}
	NewVal interface{}
}

// Model is the root Bubbletea model. It carries ALL TUI state in a single
// value; screen handlers (added in later tasks) read and write these fields
// via pointer receivers on helper methods, while Update/View use value
// receivers per the Bubbletea convention.
type Model struct {
	// state is the currently active screen.
	state appState

	// config is the loaded opencode config. May be nil when the constructor
	// was called without a config (error-tolerant launch path).
	config *config.Config

	// groupedModels maps provider name → sorted models. Never nil after
	// NewModel (range loops rely on this).
	groupedModels map[string][]opencode.Model
	// flatModels is the grouped map flattened into a single slice; the fuzzy
	// filter in model selection operates on this.
	flatModels []opencode.Model

	// --- Navigation ---

	// Each selectable screen owns its cursor so nested screens cannot overwrite
	// the selection that must be restored when returning.
	agentCursor  int
	detailCursor int
	modelCursor  int
	// selectedAgent is the agent name being edited on ScreenAgentDetail.
	selectedAgent string
	// navigationStack stores immutable screen origins for nested transitions.
	navigationStack []appState

	// --- Agent list data (populated from config in NewModel) ---

	primaryAgents  []string
	subagents      []string
	disabledAgents []string
	// editableFields is the schema shown on the Agent Detail screen.
	editableFields []string

	// --- Sub-components ---

	// filterInput is the fuzzy filter for model selection.
	filterInput textinput.Model
	// fieldInput captures typed values on ScreenFieldInput.
	fieldInput textinput.Model
	// filteredModels is the result of applying filterInput.Value() to
	// flatModels. Maintained by model_select.go in a later task.
	filteredModels []opencode.Model

	// --- State tracking ---

	// dirty is true when any in-memory edit has not yet been persisted.
	dirty bool
	// changes records every in-memory mutation since the last successful
	// save. The save-confirm screen renders this slice as a diff so the
	// user can verify what will be written to disk.
	changes []Change
	// quitConfirm is true when the "quit anyway?" confirmation overlay is
	// active on the Agent List screen. It is a sub-state of ScreenAgentList,
	// not a full screen. When true, only y/Y/ENTER (confirm) and n/N/ESC
	// (cancel) are accepted; all other keys are ignored (REQ-TUI-003).
	quitConfirm bool
	// fieldEditing records which field is being edited on ScreenFieldInput
	// (or "global" / "model" sentinel values).
	fieldEditing string
	// backupCount is the backup retention value (0 = skip backups entirely).
	backupCount int
	// saveError carries a human-readable save/validation error for display.
	saveError string
	// saveSuccess is set true for one render cycle after a successful save.
	saveSuccess bool

	// --- Terminal dimensions ---

	width  int
	height int
}

// NewModel constructs the root TUI model.
//
// The constructor is nil-safe for both cfg and grouped: a nil config skips
// agent-list population (later screens will render an error), and a nil map
// is replaced with an empty map so range loops never panic.
//
// Spec: REQ-TUI-001 — Happy path / Edge case / Error — nil config.
func NewModel(cfg *config.Config, grouped map[string][]opencode.Model, backupCount int) Model {
	// Normalize the grouped map so the rest of the code can range over it
	// unconditionally.
	if grouped == nil {
		grouped = map[string][]opencode.Model{}
	}

	m := Model{
		state:          ScreenAgentList,
		config:         cfg,
		groupedModels:  grouped,
		editableFields: append([]string(nil), editableFieldSchema...),
		backupCount:    backupCount,
	}

	// Flatten the grouped map into a single slice for the fuzzy filter. The
	// order is non-deterministic (map iteration) but the filter+sort pass in
	// model_select.go will normalize it.
	for _, models := range grouped {
		m.flatModels = append(m.flatModels, models...)
	}

	// Initialize textinput sub-components so later handlers can Update them
	// without re-allocating.
	m.filterInput = textinput.New()
	m.fieldInput = textinput.New()

	// Populate agent lists from the config when present. GetAgents already
	// filters out system agents (REQ-CFG-008) so we do not repeat that here.
	if cfg != nil {
		primary, subagents, disabled := cfg.GetAgents()
		m.primaryAgents = primary
		m.subagents = subagents
		m.disabledAgents = disabled
	}

	return m
}

func (m *Model) pushScreen(next appState) {
	m.navigationStack = append(m.navigationStack, m.state)
	m.state = next
}

func (m *Model) popScreen() {
	if len(m.navigationStack) == 0 {
		m.state = ScreenAgentList
		return
	}
	last := len(m.navigationStack) - 1
	m.state = m.navigationStack[last]
	m.navigationStack = m.navigationStack[:last]
}

// Init satisfies tea.Model. The TUI has no initial async work to schedule —
// textinput cursor blink commands are issued when an input gains focus, not
// at program start. Returning nil lets Bubbletea begin rendering immediately.
func (m Model) Init() tea.Cmd {
	return nil
}

// RecordChange appends a Change entry to the model and marks it dirty. Call
// this from every mutation site AFTER the config has been updated so OldVal
// reflects the previous value and NewVal reflects the new one.
func (m *Model) RecordChange(target, field string, oldVal, newVal interface{}) {
	m.changes = append(m.changes, Change{Target: target, Field: field, OldVal: oldVal, NewVal: newVal})
	m.dirty = true
}

// Update is the global key/message dispatcher. Ctrl+C remains global, while
// printable commands are intercepted only when no focused text input owns
// them. Screen-specific keys are routed to their handlers.
//
// Spec: REQ-TUI-003 (quit/save), REQ-TUI-007 (save trigger), REQ-TUI-008
// (ESC navigation).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// === GLOBAL KEYS (work on every screen) ===

		// If quit confirmation is active, intercept all keys
		if m.quitConfirm {
			switch msg.String() {
			case "y", "Y", "enter":
				return m, tea.Quit
			case "n", "N", "esc":
				m.quitConfirm = false
				return m, nil
			}
			return m, nil
		}

		// Ctrl+C: show quit confirmation if dirty, else quit
		if msg.Type == tea.KeyCtrlC {
			if m.dirty {
				m.quitConfirm = true
				return m, nil
			}
			return m, tea.Quit
		}

		// Focused inputs and modals own every printable key.
		if m.state == ScreenModelSelection {
			return updateModelSelection(m, msg)
		}
		if m.state == ScreenFieldInput {
			return updateFieldInput(m, msg)
		}
		if m.state == ScreenSaveConfirm {
			return updateSaveConfirm(m, msg)
		}

		// Printable q/s are commands only on non-input screens.
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'q' {
			if m.dirty {
				m.quitConfirm = true
				return m, nil
			}
			return m, tea.Quit
		}

		// 's': transition to save-confirm if dirty
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 's' {
			if m.dirty {
				m.pushScreen(ScreenSaveConfirm)
			}
			return m, nil
		}

		// === PER-SCREEN KEYS ===

		// Screen-specific key dispatch
		switch m.state {
		case ScreenAgentList:
			return updateAgentList(m, msg)
		case ScreenAgentDetail:
			return updateAgentDetail(m, msg)
		}

		return m, nil
	}

	// Non-key, non-resize messages are passed through unchanged. Sub-components
	// (textinput) will intercept their own messages in later tasks.
	return m, nil
}

// View dispatches to the per-screen renderer based on the current state.
//
// Per-screen rendering is implemented in G2-T2 through G2-T5. Until those
// land, every screen returns a non-empty placeholder so the dispatcher is
// fully exercised by tests and a manual launch does not crash.
func (m Model) View() string {
	// Error-tolerant path: if no config was supplied, surface an error
	// message instead of dereferencing a nil pointer in any screen handler.
	if m.config == nil {
		return ErrorStyle.Render("opencode-model-selector: no config loaded")
	}

	switch m.state {
	case ScreenAgentList:
		return viewAgentList(m)
	case ScreenAgentDetail:
		return viewAgentDetail(m)
	case ScreenModelSelection:
		return viewModelSelection(m)
	case ScreenFieldInput:
		return viewFieldInput(m)
	case ScreenSaveConfirm:
		return viewSaveConfirm(m)
	default:
		return ErrorStyle.Render("unknown screen state")
	}
}
