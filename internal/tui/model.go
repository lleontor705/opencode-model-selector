// Package tui implements the Bubbletea terminal UI for the opencode model
// selector using the Model-View-Update architecture.
//
// This file contains the root Model struct, the appState state machine, and
// the global Update() dispatcher. Per-screen view and key handling live in
// sibling files (agent_list.go, model_select.go, save_confirm.go) implemented
// in subsequent tasks. Until
// those land, View() returns placeholder strings so the dispatcher is fully
// exercised by tests.
//
// Spec coverage: REQ-TUI-001 (initialization), REQ-TUI-008 (transitions).
package tui

import (
	"context"
	"sort"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lleontor705/opencode-model-selector/internal/agentcatalog"
	"github.com/lleontor705/opencode-model-selector/internal/appname"
	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

const (
	minTerminalWidth  = 40
	minTerminalHeight = 12
)

// Sentinel values for fieldEditing that distinguish bulk-apply flows from
// single-agent or global edits. These are checked in selectModelAtCursor to
// route the commit logic and in popScreen to clear bulk state on exit.
const (
	fieldEditingBulkAll  = "bulk-all"
	fieldEditingBulkList = "bulk-list"
)

// appState enumerates the screens of the TUI state machine.
type appState int

const (
	// ScreenAgentList is the entry screen: lists primary agents, subagents,
	// and the global default model (REQ-TUI-002, REQ-TUI-003).
	ScreenAgentList appState = iota
	// ScreenModelSelection shows all available models grouped by provider
	// with a fuzzy filter (REQ-TUI-005).
	ScreenModelSelection
	// ScreenSaveConfirm shows a summary of pending changes and triggers the
	// atomic save flow (REQ-TUI-007).
	ScreenSaveConfirm
	// ScreenAgentMultiSelect shows a checkbox list of agents for Flow B
	// bulk operations. ENTER transitions to ScreenModelSelection with
	// fieldEditing="bulk-list".
	ScreenAgentMultiSelect
	// ScreenVariantSelection shows available variants for the selected model.
	ScreenVariantSelection
)

// Change records one model or variant mutation pending persistence. Target is
// "global" or a catalog agent name; no generic config field is representable.
type Change struct {
	Target     string
	OldModel   string
	NewModel   string
	OldVariant string
	NewVariant string
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
	agentCursor   int
	modelCursor   int
	variantCursor int
	// selectedAgent is the agent whose model is being edited.
	selectedAgent string
	// pendingSelectedModel is the model chosen in ScreenModelSelection awaiting variant selection.
	pendingSelectedModel opencode.Model
	// availableVariants holds the compatible variants for pendingSelectedModel.
	availableVariants []opencode.VariantDescriptor
	// navigationStack stores immutable screen origins for nested transitions.
	navigationStack []appState

	// --- Agent list data (populated from config in NewModel) ---

	primaryAgents  []string
	subagents      []string
	allAgents      []string
	disabledAgents []string
	agentCatalog   agentcatalog.Catalog
	catalogByName  map[string]agentcatalog.AgentRecord
	// mdOnlyAgents is the set of agent names that currently exist ONLY in
	// markdown (no inline-JSON backing). It is recomputed by NewModel and by
	// performSave so the [MD] badge stays live: once a JSON override is saved
	// for an agent, it leaves this set. The badge hints that the base is a
	// markdown file; it does NOT mean the agent is non-editable — ENTER opens
	// the model picker and model edits persist as inline-JSON overrides. The .md
	// file is never written.
	mdOnlyAgents map[string]bool

	// --- Sub-components ---

	// filterInput is the fuzzy filter for model selection.
	filterInput textinput.Model
	// Each scrolling screen owns a Bubbles viewport. The existing screen-owned
	// cursors remain the source of truth; viewport offsets only control clipping.
	agentViewport   viewport.Model
	modelViewport   viewport.Model
	variantViewport viewport.Model
	saveViewport    viewport.Model
	// filteredModels is the result of applying filterInput.Value() to
	// flatModels. Maintained by model_select.go in a later task.
	filteredModels []opencode.Model

	// --- State tracking ---

	// dirty is true when any in-memory edit has not yet been persisted.
	dirty bool
	// changes records net model mutations since the last successful save,
	// coalesced by target for review before writing to disk.
	changes []Change
	// quitConfirm is true when the "quit anyway?" confirmation overlay is
	// active on the Agent List screen. It is a sub-state of ScreenAgentList,
	// not a full screen. When true, only y/Y/ENTER (confirm) and n/N/ESC
	// (cancel) are accepted; all other keys are ignored (REQ-TUI-003).
	quitConfirm bool
	// fieldEditing identifies the model assignment target or bulk flow.
	fieldEditing string
	// bulkTargets holds the agent names selected on ScreenAgentMultiSelect.
	// Populated when transitioning to ScreenModelSelection with
	// fieldEditing="bulk-list". For fieldEditing="bulk-all" it stays nil
	// because targets are resolved at commit time via GetAgents.
	bulkTargets []string
	// multiSelectItems is the cached list of selectable agent names for
	// ScreenAgentMultiSelect (primary + subagents minus disabled, sorted).
	multiSelectItems []string
	// multiSelectChecked is parallel to multiSelectItems; true = selected.
	multiSelectChecked []bool
	// multiSelectCursor is the cursor row on ScreenAgentMultiSelect.
	multiSelectCursor int
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
	// Live discovery is wired by cmd in T13. Preserve existing callers with a
	// deterministic, degraded static catalog until that integration lands.
	var catalog agentcatalog.Catalog
	if cfg == nil {
		catalog = agentcatalog.Discovery{}.Discover(context.Background(), "")
	} else {
		catalog = agentcatalog.Discovery{Static: cfg}.Discover(context.Background(), "")
	}
	return NewModelWithCatalog(cfg, grouped, backupCount, catalog)
}

// NewModelWithCatalog constructs the TUI from an already discovered catalog.
// Discovery and process lifecycle concerns intentionally remain outside TUI.
func NewModelWithCatalog(cfg *config.Config, grouped map[string][]opencode.Model, backupCount int, catalog agentcatalog.Catalog) Model {
	// Normalize the grouped map so the rest of the code can range over it
	// unconditionally.
	if grouped == nil {
		grouped = map[string][]opencode.Model{}
	}

	catalog.Buckets = agentcatalog.Classify(catalog.Records())
	m := Model{
		state:         ScreenAgentList,
		config:        cfg,
		groupedModels: grouped,
		backupCount:   backupCount,
		agentCatalog:  catalog,
		catalogByName: make(map[string]agentcatalog.AgentRecord),
	}

	// Flatten the grouped map into a single slice for the fuzzy filter. The
	// order is non-deterministic (map iteration) but the filter+sort pass in
	// model_select.go will normalize it.
	for _, models := range grouped {
		m.flatModels = append(m.flatModels, models...)
	}
	if cfg != nil && cfg.Data() != nil {
		for i := range m.flatModels {
			if len(m.flatModels[i].Variants) == 0 {
				m.flatModels[i].Variants = opencode.ExtractModelVariants(cfg.Data(), m.flatModels[i].Provider, m.flatModels[i].ID)
			}
		}
	}

	// Initialize textinput sub-components so later handlers can Update them
	// without re-allocating.
	m.filterInput = textinput.New()
	m.agentViewport = viewport.New()
	m.modelViewport = viewport.New()
	m.variantViewport = viewport.New()
	m.saveViewport = viewport.New()

	for _, record := range catalog.Buckets.Primary {
		m.primaryAgents = append(m.primaryAgents, record.Name)
		m.catalogByName[record.Name] = record
	}
	for _, record := range catalog.Buckets.Subagent {
		m.subagents = append(m.subagents, record.Name)
		m.catalogByName[record.Name] = record
	}
	for _, record := range catalog.Buckets.All {
		m.allAgents = append(m.allAgents, record.Name)
		m.catalogByName[record.Name] = record
	}

	// Editing metadata remains config-backed until action routing changes.
	if cfg != nil {
		_, _, disabled := cfg.GetAgents()
		m.disabledAgents = disabled
		m.mdOnlyAgents = computeMdOnly(cfg)
	}

	return m
}

// computeMdOnly returns the set of agent names that are markdown-only (no
// inline-JSON backing), derived from the config's merged agent view.
func computeMdOnly(cfg *config.Config) map[string]bool {
	out := map[string]bool{}
	if cfg == nil {
		return out
	}
	for name, merged := range cfg.MergedAgents() {
		if merged.MdOnly {
			out[name] = true
		}
	}
	return out
}

// IsMarkdownOnly reports whether name is currently a markdown-only agent (no
// inline-JSON backing yet). This is informational only — used to render the
// [MD] badge hint. It does NOT gate editing: markdown-backed agents are
// editable, and model edits persist as inline-JSON overrides.
func (m Model) IsMarkdownOnly(name string) bool {
	return m.mdOnlyAgents[name]
}

// selectableCatalogNames returns each editable catalog identity exactly once,
// across primary, subagent, and all-role buckets.
func selectableCatalogNames(m Model) []string {
	disabled := make(map[string]struct{}, len(m.disabledAgents))
	for _, name := range m.disabledAgents {
		disabled[name] = struct{}{}
	}
	seen := make(map[string]struct{}, len(m.catalogByName))
	names := make([]string, 0, len(m.catalogByName))
	for _, record := range m.agentCatalog.Records() {
		if record.Name == "" {
			continue
		}
		if _, excluded := disabled[record.Name]; excluded {
			continue
		}
		if _, duplicate := seen[record.Name]; duplicate {
			continue
		}
		seen[record.Name] = struct{}{}
		names = append(names, record.Name)
	}
	sort.Strings(names)
	return names
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
	if m.state == ScreenVariantSelection {
		m.pendingSelectedModel = opencode.Model{}
		m.availableVariants = nil
		m.variantCursor = 0
	}
	if m.state == ScreenModelSelection &&
		(m.fieldEditing == fieldEditingBulkAll || m.fieldEditing == fieldEditingBulkList) {
		m.fieldEditing = ""
		m.bulkTargets = nil
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

// RecordModelChange coalesces model mutations by target. The first old model
// is retained while later edits replace the pending new model. Reverting to
// the original model removes the net change entirely.
func (m *Model) RecordModelChange(target, oldModel, newModel string) {
	for i := range m.changes {
		change := &m.changes[i]
		if change.Target != target {
			continue
		}
		change.NewModel = newModel
		if change.OldModel == change.NewModel && change.OldVariant == change.NewVariant {
			m.changes = append(m.changes[:i], m.changes[i+1:]...)
		}
		m.dirty = len(m.changes) > 0
		return
	}

	if oldModel == newModel {
		m.dirty = len(m.changes) > 0
		return
	}
	m.changes = append(m.changes, Change{Target: target, OldModel: oldModel, NewModel: newModel})
	m.dirty = true
}

// RecordChange coalesces independent model and variant mutations by target.
// Reverting both model and variant to their original values removes the net change.
func (m *Model) RecordChange(target, oldModel, newModel, oldVariant, newVariant string) {
	for i := range m.changes {
		change := &m.changes[i]
		if change.Target != target {
			continue
		}
		change.NewModel = newModel
		change.NewVariant = newVariant
		if change.OldModel == change.NewModel && change.OldVariant == change.NewVariant {
			m.changes = append(m.changes[:i], m.changes[i+1:]...)
		}
		m.dirty = len(m.changes) > 0
		return
	}

	if oldModel == newModel && oldVariant == newVariant {
		m.dirty = len(m.changes) > 0
		return
	}
	m.changes = append(m.changes, Change{
		Target:     target,
		OldModel:   oldModel,
		NewModel:   newModel,
		OldVariant: oldVariant,
		NewVariant: newVariant,
	})
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
		m.filterInput.SetWidth(max(1, msg.Width-lipgloss.Width("🔍 Search: ")-1))
		m.agentViewport.SetWidth(max(1, msg.Width))
		m.agentViewport.SetHeight(agentListViewportHeight(m))
		m.modelViewport.SetWidth(max(1, msg.Width))
		m.modelViewport.SetHeight(modelSelectionViewportHeight(m))
		m.variantViewport.SetWidth(max(1, msg.Width))
		m.variantViewport.SetHeight(variantSelectionViewportHeight(m))
		m.saveViewport.SetWidth(max(1, msg.Width))
		m.saveViewport.SetHeight(saveReviewViewportHeight(m))
		syncAgentViewport(&m)
		syncModelViewport(&m)
		syncVariantViewport(&m)
		syncSaveViewport(&m)
		return m, nil

	case tea.KeyPressMsg:
		// Bubble Tea renders after every Update. A successful save therefore gets
		// one complete Agent List frame before the next user action clears it.
		if m.saveSuccess {
			m.saveSuccess = false
		}
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
		if msg.Code == 'c' && msg.Mod.Contains(tea.ModCtrl) {
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
		if m.state == ScreenVariantSelection {
			return updateVariantSelection(m, msg)
		}
		if m.state == ScreenSaveConfirm {
			return updateSaveConfirm(m, msg)
		}
		if m.state == ScreenAgentMultiSelect {
			updatedM, cmd := updateAgentMultiSelect(m, msg)
			if m.state != ScreenModelSelection && updatedM.state == ScreenModelSelection && cmd == nil {
				cmd = updatedM.filterInput.Focus()
			}
			return updatedM, cmd
		}

		// Printable q/s are commands only on non-input screens.
		if msg.Text == "q" {
			if m.dirty {
				m.quitConfirm = true
				return m, nil
			}
			return m, tea.Quit
		}

		// 's': transition to save-confirm if dirty
		if msg.Text == "s" {
			if m.dirty {
				m.pushScreen(ScreenSaveConfirm)
			}
			return m, nil
		}

		// === PER-SCREEN KEYS ===

		// Screen-specific key dispatch
		switch m.state {
		case ScreenAgentList:
			updatedM, cmd := updateAgentList(m, msg)
			if m.state != ScreenModelSelection && updatedM.state == ScreenModelSelection && cmd == nil {
				cmd = updatedM.filterInput.Focus()
			}
			return updatedM, cmd
		}

		return m, nil
	}

	// Non-key, non-resize messages are routed to active screens/sub-components.
	if m.state == ScreenModelSelection {
		return updateModelSelection(m, msg)
	}
	if m.state == ScreenVariantSelection {
		return updateVariantSelection(m, msg)
	}

	return m, nil
}

// View dispatches to the per-screen renderer based on the current state and
// returns a Bubble Tea v2 View whose Content carries the rendered screen body
// and whose AltScreen flag keeps the TUI in the alternate screen buffer.
func (m Model) View() tea.View {
	return tea.View{Content: m.renderBody(), AltScreen: true}
}

// renderBody builds the plain-text screen body. Error and terminal-too-small
// paths flow through the same wrapper as the regular screens.
func (m Model) renderBody() string {
	// Error-tolerant path: if no config was supplied, surface an error
	// message instead of dereferencing a nil pointer in any screen handler.
	if m.config == nil {
		return ErrorStyle.Render(appname.Name + ": no config loaded")
	}
	if m.width > 0 && m.height > 0 &&
		(m.width < minTerminalWidth || m.height < minTerminalHeight) {
		return renderTerminalTooSmall(m.width, m.height)
	}

	switch m.state {
	case ScreenAgentList:
		return viewAgentList(m)
	case ScreenModelSelection:
		return viewModelSelection(m)
	case ScreenVariantSelection:
		return viewVariantSelection(m)
	case ScreenSaveConfirm:
		return viewSaveConfirm(m)
	case ScreenAgentMultiSelect:
		return viewAgentMultiSelect(m)
	default:
		return ErrorStyle.Render("unknown screen state")
	}
}

func ensureViewportRange(vp *viewport.Model, start, end int) {
	if vp.Height() <= 0 {
		return
	}
	if start < vp.YOffset() {
		vp.SetYOffset(start)
		return
	}
	if end >= vp.YOffset()+vp.Height() {
		vp.SetYOffset(max(0, end-vp.Height()+1))
	}
}
