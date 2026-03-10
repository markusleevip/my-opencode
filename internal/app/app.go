package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"myopencode/internal/config"
	"myopencode/internal/db"
	"myopencode/internal/format"
	"myopencode/internal/history"
	"myopencode/internal/llm/agent"
	"myopencode/internal/logging"
	"myopencode/internal/lsp"
	"myopencode/internal/message"
	"myopencode/internal/permission"
	"myopencode/internal/session"
	"myopencode/internal/skills"
	"myopencode/internal/tui/theme"
)

type App struct {
	Sessions    session.Service
	Messages    message.Service
	History     history.Service
	Permissions permission.Service
	Skills      *skills.Manager

	CoderAgent agent.Service

	LSPClients map[string]*lsp.Client

	clientsMutex sync.RWMutex

	watcherCancelFuncs []context.CancelFunc
	cancelFuncsMutex   sync.Mutex
	watcherWG          sync.WaitGroup
}

func New(ctx context.Context, conn *sql.DB) (*App, error) {
	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)
	files := history.NewService(q, conn)

	app := &App{
		Sessions:    sessions,
		Messages:    messages,
		History:     files,
		Permissions: permission.NewPermissionService(),
		Skills:      nil, // Will be initialized below
		LSPClients:  make(map[string]*lsp.Client),
	}

	// Initialize skills manager (auto-discovers and loads skills)
	skillsMgr, err := skills.NewManager(config.Get().WorkingDir)
	if err != nil {
		logging.Warn("Failed to initialize skills manager", "error", err)
	} else {
		app.Skills = skillsMgr
		logging.Info("Skills manager initialized", "count", skillsMgr.Count())
	}

	// Initialize theme based on configuration
	app.initTheme()

	// Initialize LSP clients in the background
	go app.initLSPClients(ctx)

	app.CoderAgent, err = agent.NewAgent(
		config.AgentCoder,
		app.Sessions,
		app.Messages,
		agent.CoderAgentTools(
			app.Permissions,
			app.Sessions,
			app.Messages,
			app.History,
			app.LSPClients,
		),
	)
	if err != nil {
		logging.Error("Failed to create coder agent", err)
		return nil, err
	}

	// Set skills manager on the agent (if available)
	if app.Skills != nil {
		app.CoderAgent.SetSkillsManager(&skillsAgentWrapper{manager: app.Skills})
	}

	return app, nil
}

// skillsAgentWrapper adapts skills.Manager to agent.SkillsManager interface
type skillsAgentWrapper struct {
	manager *skills.Manager
}

func (w *skillsAgentWrapper) MatchSkills(input string) []agent.SkillMatchResult {
	matches := w.manager.MatchSkills(input)
	if len(matches) == 0 {
		return nil
	}

	// Convert skills.MatchResult to agent.SkillMatchResult
	result := make([]agent.SkillMatchResult, len(matches))
	for i, m := range matches {
		result[i] = agent.SkillMatchResult{
			SkillName: m.SkillName,
			Score:     m.Score,
			Reason:    m.Reason,
		}
	}
	return result
}

func (w *skillsAgentWrapper) GetSkillContext(matches []agent.SkillMatchResult) string {
	if len(matches) == 0 {
		return ""
	}

	// Convert agent.SkillMatchResult back to skills.MatchResult
	skillMatches := make([]skills.MatchResult, len(matches))
	for i, m := range matches {
		skillMatches[i] = skills.MatchResult{
			SkillName: m.SkillName,
			Score:     m.Score,
			Reason:    m.Reason,
		}
	}

	// Use the manager's GetSkillContext which handles the index internally
	return w.manager.GetSkillContext(skillMatches)
}

func (w *skillsAgentWrapper) GetSkillsSummary() string {
	return w.manager.BuildSkillsSummary()
}

// initTheme sets the application theme based on the configuration
func (app *App) initTheme() {
	cfg := config.Get()
	if cfg == nil || cfg.TUI.Theme == "" {
		return // Use default theme
	}

	// Try to set the theme from config
	err := theme.SetTheme(cfg.TUI.Theme)
	if err != nil {
		logging.Warn("Failed to set theme from config, using default theme", "theme", cfg.TUI.Theme, "error", err)
	} else {
		logging.Debug("Set theme from config", "theme", cfg.TUI.Theme)
	}
}

// RunNonInteractive handles the execution flow when a prompt is provided via CLI flag.
func (a *App) RunNonInteractive(ctx context.Context, prompt string, outputFormat string, quiet bool) error {
	logging.Info("Running in non-interactive mode")

	// Start spinner if not in quiet mode
	var spinner *format.Spinner
	if !quiet {
		spinner = format.NewSpinner("Thinking...")
		spinner.Start()
		defer spinner.Stop()
	}

	const maxPromptLengthForTitle = 100
	titlePrefix := "Non-interactive: "
	var titleSuffix string

	if len(prompt) > maxPromptLengthForTitle {
		titleSuffix = prompt[:maxPromptLengthForTitle] + "..."
	} else {
		titleSuffix = prompt
	}
	title := titlePrefix + titleSuffix

	sess, err := a.Sessions.Create(ctx, title)
	if err != nil {
		return fmt.Errorf("failed to create session for non-interactive mode: %w", err)
	}
	logging.Info("Created session for non-interactive run", "session_id", sess.ID)

	// Automatically approve all permission requests for this non-interactive session
	a.Permissions.AutoApproveSession(sess.ID)

	done, err := a.CoderAgent.Run(ctx, sess.ID, prompt)
	if err != nil {
		return fmt.Errorf("failed to start agent processing stream: %w", err)
	}

	result := <-done
	if result.Error != nil {
		if errors.Is(result.Error, context.Canceled) || errors.Is(result.Error, agent.ErrRequestCancelled) {
			logging.Info("Agent processing cancelled", "session_id", sess.ID)
			return nil
		}
		return fmt.Errorf("agent processing failed: %w", result.Error)
	}

	// Stop spinner before printing output
	if !quiet && spinner != nil {
		spinner.Stop()
	}

	// Get the text content from the response
	content := "No content available"
	if result.Message.Content().String() != "" {
		content = result.Message.Content().String()
	}

	fmt.Println(format.FormatOutput(content, outputFormat))

	logging.Info("Non-interactive run completed", "session_id", sess.ID)

	return nil
}

// Shutdown performs a clean shutdown of the application
func (app *App) Shutdown() {
	// Cancel all watcher goroutines
	app.cancelFuncsMutex.Lock()
	for _, cancel := range app.watcherCancelFuncs {
		cancel()
	}
	app.cancelFuncsMutex.Unlock()
	app.watcherWG.Wait()

	// Perform additional cleanup for LSP clients
	app.clientsMutex.RLock()
	clients := make(map[string]*lsp.Client, len(app.LSPClients))
	maps.Copy(clients, app.LSPClients)
	app.clientsMutex.RUnlock()

	for name, client := range clients {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := client.Shutdown(shutdownCtx); err != nil {
			logging.Error("Failed to shutdown LSP client", "name", name, "error", err)
		}
		cancel()
	}
}
