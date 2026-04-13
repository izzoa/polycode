package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/action"
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/hooks"
	"github.com/izzoa/polycode/internal/mcp"
	"github.com/izzoa/polycode/internal/memory"
	"github.com/izzoa/polycode/internal/permissions"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/routing"
	"github.com/izzoa/polycode/internal/skill"
	"github.com/izzoa/polycode/internal/telemetry"
	"github.com/izzoa/polycode/internal/tokens"
	"github.com/izzoa/polycode/internal/tui"
)

// mcpHolder provides thread-safe access to the shared MCPClient pointer.
// Reads use RLock (concurrent), writes use Lock (exclusive).
type mcpHolder struct {
	mu     sync.RWMutex
	client *mcp.MCPClient
}

func (h *mcpHolder) get() *mcp.MCPClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.client
}

func (h *mcpHolder) set(c *mcp.MCPClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.client = c
}

// appState bundles mutable application state that can be replaced atomically
// when the config changes. Handlers call state.Load() once at the top to get
// a consistent snapshot for their entire lifetime — no partial state possible.
type appState struct {
	tracker   *tokens.TokenTracker
	registry  *provider.Registry
	healthy   []provider.Provider
	primary   provider.Provider
	cfg       *config.Config
	hookMgr   *hooks.HookManager
	policyMgr *permissions.PolicyManager
}

// conversationState maintains the full multi-turn dialogue context.
type conversationState struct {
	mu       sync.Mutex
	messages []provider.Message
}

func (c *conversationState) append(msgs ...provider.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msgs...)
}

func (c *conversationState) snapshot() []provider.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]provider.Message, len(c.messages))
	copy(out, c.messages)
	return out
}

func startTUI(cfg *config.Config) error {
	// Create provider registry
	registry, err := provider.NewRegistry(cfg)
	if err != nil {
		return fmt.Errorf("creating provider registry: %w", err)
	}

	// Authenticate all providers
	for _, p := range registry.Providers() {
		if err := p.Authenticate(); err != nil {
			fmt.Printf("Warning: failed to authenticate %s: %v\n", p.ID(), err)
		}
	}

	// Get healthy providers
	healthy := registry.Healthy()
	if len(healthy) == 0 {
		return fmt.Errorf("no healthy providers available — run 'polycode auth login <provider>' to authenticate")
	}

	primary := registry.Primary()
	if err := primary.Validate(); err != nil {
		return fmt.Errorf("primary provider %s is not healthy: %w", primary.ID(), err)
	}

	// Create metadata store for litellm model metadata
	cachePath := filepath.Join(config.ConfigDir(), "model_metadata.json")
	metadataStore, err := tokens.NewMetadataStore(
		cfg.Metadata.URL,
		cachePath,
		cfg.Metadata.CacheTTL,
	)
	if err != nil {
		log.Printf("Warning: failed to initialize metadata store: %v", err)
	}

	// Build provider name list and resolve token limits
	var names []string
	providerModels := make(map[string]string)
	providerLimits := make(map[string]int)
	for _, pc := range cfg.Providers {
		providerModels[pc.Name] = pc.Model
		if metadataStore != nil {
			providerLimits[pc.Name] = metadataStore.LimitForModel(pc.Model, string(pc.Type), pc.MaxContext)
		} else {
			providerLimits[pc.Name] = tokens.LimitForModel(pc.Model, pc.MaxContext)
		}
	}
	for _, p := range healthy {
		names = append(names, p.ID())
	}

	// Create token tracker
	tracker := tokens.NewTracker(providerModels, providerLimits)

	// Wire cost estimation from litellm pricing data
	if metadataStore != nil {
		tracker.SetCostFunc(func(model, providerType string, inputTokens, outputTokens int) float64 {
			return metadataStore.CostForTokens(model, providerType, inputTokens, outputTokens)
		})
	}
	for _, pc := range cfg.Providers {
		tracker.SetProviderType(pc.Name, string(pc.Type))
	}

	// Create telemetry logger
	tlog, err := telemetry.NewLogger()
	if err != nil {
		log.Printf("Warning: telemetry disabled: %v", err)
	}
	if tlog != nil {
		defer tlog.Close()
	}

	// Working directory for repo-level config (instructions, permissions)
	workDir, _ := os.Getwd()

	// Create hook manager
	hookMgr := hooks.NewHookManager(cfg.Hooks)

	// Load permission policies (repo-level overrides user-level)
	policyMgr, err := permissions.LoadPolicies(workDir)
	if err != nil {
		log.Printf("Warning: failed to load permission policies: %v", err)
		// Create an empty policy manager so Check() returns PolicyAsk for everything
		policyMgr, _ = permissions.LoadPolicies("")
	}

	// Create adaptive router
	telemetryPath := filepath.Join(config.ConfigDir(), "telemetry.jsonl")
	router := routing.NewRouter(telemetryPath)
	if err := router.LoadTelemetryStats(); err != nil {
		log.Printf("Warning: failed to load telemetry stats for router: %v", err)
	}

	// Parse initial operating mode (guarded by mutex for goroutine safety)
	var modeMu sync.Mutex
	currentMode := routing.ModeBalanced
	if m, ok := routing.ParseMode(cfg.DefaultMode); ok {
		currentMode = m
	}
	getMode := func() routing.Mode {
		modeMu.Lock()
		defer modeMu.Unlock()
		return currentMode
	}
	setMode := func(m routing.Mode) {
		modeMu.Lock()
		defer modeMu.Unlock()
		currentMode = m
	}

	// Select initial providers based on mode
	routed := router.SelectProviders(getMode(), healthy, primary.ID())
	if len(routed) == 0 {
		routed = healthy // fallback
	}

	// Create repo memory store
	memDir := filepath.Join(config.ConfigDir(), "memory")
	memStore := memory.NewMemoryStore(memDir)

	// Build system prompt from instruction hierarchy + repo memory + project context
	instructions := memory.LoadInstructions(workDir)
	memPrompt := memStore.FormatForPrompt()
	systemContent := instructions
	if memPrompt != "" {
		systemContent += "\n\n" + memPrompt
	}

	// Inject project context and tool hints so providers don't waste rounds exploring.
	projectCtx := action.BuildProjectContext(workDir)
	toolHints := action.ToolUsageHints()
	systemContent += "\n\n" + projectCtx + "\n" + toolHints

	// Connect to MCP servers and discover tools
	mcpH := &mcpHolder{}
	if len(cfg.MCP.Servers) > 0 {
		newMCP := mcp.NewMCPClient(cfg.MCP.Servers, cfg.MCP.Debug)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := newMCP.Connect(ctx); err != nil {
			log.Printf("Warning: MCP: %v", err)
		}
		cancel()
		if len(newMCP.Tools()) == 0 {
			newMCP.Close()
		} else {
			mcpH.set(newMCP)
		}
	}

	// Load installed skills
	skillsDir := filepath.Join(config.ConfigDir(), "skills")
	skillReg := skill.NewRegistry(skillsDir)
	if err := skillReg.Load(); err != nil {
		log.Printf("Warning: failed to load skills: %v", err)
	}

	// Append skill system prompts to system content
	if skillPrompts := skillReg.SystemPrompts(); skillPrompts != "" {
		systemContent += "\n\n" + skillPrompts
	}

	// Note: no static pipeline variable — providers are selected per query
	// by the router in the submit handler. Mode/config changes update the
	// router inputs (healthy, primary, cfg) which take effect on the next query.

	// System prompt built from instruction hierarchy + repo memory
	systemPrompt := provider.Message{
		Role:    provider.RoleSystem,
		Content: systemContent,
	}

	// Conversation state persists across turns
	conv := &conversationState{
		messages: []provider.Message{systemPrompt},
	}

	// Bundle shared mutable state into an atomic pointer for safe concurrent access.
	// After this point, handlers must use state.Load() instead of bare variable names.
	var state atomic.Pointer[appState]
	state.Store(&appState{
		tracker:   tracker,
		registry:  registry,
		healthy:   healthy,
		primary:   primary,
		cfg:       cfg,
		hookMgr:   hookMgr,
		policyMgr: policyMgr,
	})

	// Create TUI model
	model := tui.NewModel(names, primary.ID(), version)

	// Initialize file index for @ file references
	model.InitFileIndex(workDir)

	// Desktop notifications are opt-in (config-driven, not auto-enabled)
	// to avoid leaking sensitive tool details to the OS notification center.
	// Users can enable via config or future /notify command.

	// Task 4.1/4.2: Pass model listing closure to the TUI model for wizard use
	model.SetModelLister(func(providerType string) []config.ModelSummary {
		if metadataStore == nil {
			return nil
		}
		return metadataStore.ModelsForProvider(providerType)
	})

	// Auto-resume: load saved session if one exists (single load, reused for splash + restore)
	if savedSession, err := config.LoadSession(); err == nil && savedSession != nil {
		// Show session info on splash screen
		if len(savedSession.Exchanges) > 0 {
			ago := time.Since(savedSession.UpdatedAt)
			var agoStr string
			switch {
			case ago < time.Hour:
				agoStr = fmt.Sprintf("%dm ago", int(ago.Minutes()))
			case ago < 24*time.Hour:
				agoStr = fmt.Sprintf("%dh ago", int(ago.Hours()))
			default:
				agoStr = fmt.Sprintf("%dd ago", int(ago.Hours()/24))
			}
			model.SetSplashSessionInfo(fmt.Sprintf("Resuming session from %s, %d exchanges", agoStr, len(savedSession.Exchanges)))
		}
		// Restore session name if set
		if savedSession.Name != "" {
			model.SetSessionName(savedSession.Name)
		}
		if len(savedSession.Messages) > 0 {
			// Restore conversation messages with full tool call data
			restored := fromSessionMessages(savedSession.Messages)
			conv.mu.Lock()
			conv.messages = []provider.Message{systemPrompt}
			for _, m := range restored {
				if m.Role == provider.RoleSystem {
					continue // skip saved system prompt, we use the current one
				}
				conv.messages = append(conv.messages, m)
			}
			conv.mu.Unlock()
		}

		// Restore display history (independent of Messages — works with older formats)
		for _, ex := range savedSession.Exchanges {
			tuiEx := tui.Exchange{
				Prompt:             ex.Prompt,
				ConsensusResponse:  ex.ConsensusResponse,
				IndividualResponse: ex.Individual,
				ProviderOrder:      ex.ProviderOrder,
				PrimaryProvider:    ex.PrimaryProvider,
			}
			if len(ex.ProviderStatuses) > 0 {
				tuiEx.ProviderStatuses = make(map[string]tui.ProviderStatus, len(ex.ProviderStatuses))
				for name, statusStr := range ex.ProviderStatuses {
					tuiEx.ProviderStatuses[name] = tui.ParseProviderStatus(statusStr)
				}
			}
			if len(ex.ProviderTraces) > 0 {
				tuiEx.ProviderTraces = make(map[string][]tui.TraceSection)
				for provName, sections := range ex.ProviderTraces {
					tuiSections := make([]tui.TraceSection, len(sections))
					for i, s := range sections {
						tuiSections[i] = tui.TraceSection{Phase: s.Phase, Content: s.Content}
					}
					tuiEx.ProviderTraces[provName] = tuiSections
				}
			}
			model.AppendHistory(tuiEx)
		}
		// Populate provider panels with the last exchange's content
		// so individual responses are visible on resume, not just consensus.
		model.RestorePanelsFromLastExchange()
	}
	model.SetConfig(cfg)

	// Declare program early so handler closures can capture it.
	// It's set after NewProgram but before Run(), so it's always
	// non-nil by the time any handler goroutine executes.
	var program *tea.Program

	// Track yolo mode for auto-approve (atomic for goroutine safety)
	var yoloEnabled atomic.Bool

	// Git-backed undo/redo
	var undoTagCounter int
	isGitRepo := func() bool {
		cmd := exec.Command("git", "rev-parse", "--git-dir")
		cmd.Dir = workDir
		return cmd.Run() == nil
	}()

	// Snapshot helper: creates a lightweight git snapshot before mutating tools.
	// Records the current state so undo can restore from it.
	createUndoSnapshot := func(description string) {
		if !isGitRepo {
			return
		}
		undoTagCounter++
		// Stage and commit current state as a checkpoint
		addCmd := exec.Command("git", "add", "-A")
		addCmd.Dir = workDir
		_ = addCmd.Run()

		tag := fmt.Sprintf("polycode-undo-%d", undoTagCounter)
		commitCmd := exec.Command("git", "commit", "--allow-empty", "-m",
			fmt.Sprintf("polycode checkpoint: %s", description))
		commitCmd.Dir = workDir
		if err := commitCmd.Run(); err == nil {
			// Tag the checkpoint so we can find it later
			tagCmd := exec.Command("git", "tag", "-f", tag)
			tagCmd.Dir = workDir
			_ = tagCmd.Run()

			program.Send(tui.UndoSnapshotMsg{
				Snapshot: tui.UndoSnapshot{
					Tag:         tag,
					Description: description,
				},
			})
		}
	}

	cancelFuncs := &cancelTracker{}

	// Wire all handler groups (defined in app_*.go files)
	wireConfigChangeHandler(&model, &program, &state, mcpH, metadataStore, workDir)
	wireGeneralHandlers(
		&model, &program, &state, conv, mcpH, setMode, skillReg, memStore,
		workDir, systemPrompt, &yoloEnabled, isGitRepo, cancelFuncs,
	)
	wireMCPHandlers(&model, &program, &state, mcpH)
	wireSessionHandlers(&model, &program, &state)
	wireQueryHandler(
		&model, &program, &state, conv, mcpH, router, getMode, skillReg,
		metadataStore, tlog, systemPrompt, workDir, &yoloEnabled,
		createUndoSnapshot, cancelFuncs,
	)

	// Create the Bubble Tea program AFTER all handlers are wired,
	// so the model copy Bubble Tea receives has all callbacks set.
	program = tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Send initial MCP status to the TUI and wire notification handler.
	if mc := mcpH.get(); mc != nil {
		go sendMCPStatus(program, mc)
		wireMCPNotifications(program, mc)
	}

	// Run the TUI
	if _, err := program.Run(); err != nil {
		if mc := mcpH.get(); mc != nil {
			mc.Close()
		}
		return fmt.Errorf("TUI error: %w", err)
	}

	// Clean up MCP connections
	if mc := mcpH.get(); mc != nil {
		mc.Close()
	}

	return nil
}
