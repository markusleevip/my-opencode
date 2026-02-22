// Package config manages application configuration from various sources.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"myopencode/internal/llm/models"
	"myopencode/internal/logging"

	"github.com/spf13/viper"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MCPType defines the type of MCP (Model Control Protocol) server.
type MCPType string

// Supported MCP types
const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
)

// MCPServer defines the configuration for a Model Control Protocol server.
type MCPServer struct {
	Command string            `json:"command" mapstructure:"command"`
	Env     []string          `json:"env" mapstructure:"env"`
	Args    []string          `json:"args" mapstructure:"args"`
	Type    MCPType           `json:"type" mapstructure:"type"`
	URL     string            `json:"url" mapstructure:"url"`
	Headers map[string]string `json:"headers" mapstructure:"headers"`
}

type AgentName string

const (
	AgentCoder      AgentName = "coder"
	AgentSummarizer AgentName = "summarizer"
	AgentTask       AgentName = "task"
	AgentTitle      AgentName = "title"
)

// Agent defines configuration for different LLM models and their token limits.
type Agent struct {
	Model           models.ModelID `json:"model" mapstructure:"model"`
	MaxTokens       int64          `json:"maxTokens" mapstructure:"maxTokens"`
	ReasoningEffort string         `json:"reasoningEffort" mapstructure:"reasoningEffort"`
}

// Provider defines configuration for an LLM provider.
type Provider struct {
	APIKey   string `json:"apiKey" mapstructure:"apiKey"`
	BaseURL  string `json:"baseURL,omitempty" mapstructure:"baseURL"`
	Disabled bool   `json:"disabled" mapstructure:"disabled"`
}

// DynamicModelConfig defines a model entry in a dynamic provider definition.
type DynamicModelConfig struct {
	Name string `json:"name" mapstructure:"name"`
}

// DynamicProviderOptions defines the options block for a dynamic provider.
type DynamicProviderOptions struct {
	BaseURL string `json:"baseURL" mapstructure:"baseURL"`
	APIKey  string `json:"apiKey" mapstructure:"apiKey"`
}

// DynamicProviderDef defines an OpenCode-style provider definition.
// Supports npm = "@ai-sdk/openai-compatible" for OpenAI-compatible endpoints.
type DynamicProviderDef struct {
	NPM     string                        `json:"npm" mapstructure:"npm"`
	Name    string                        `json:"name" mapstructure:"name"`
	APIKey  string                        `json:"apiKey" mapstructure:"apiKey"`
	Options DynamicProviderOptions        `json:"options" mapstructure:"options"`
	Models  map[string]DynamicModelConfig `json:"models" mapstructure:"models"`
}

// Data defines storage configuration.
type Data struct {
	Directory string `json:"directory,omitempty" mapstructure:"directory"`
}

// LSPConfig defines configuration for Language Server Protocol integration.
type LSPConfig struct {
	Disabled bool     `json:"enabled" mapstructure:"enabled"`
	Command  string   `json:"command" mapstructure:"command"`
	Args     []string `json:"args" mapstructure:"args"`
	Options  any      `json:"options" mapstructure:"options"`
}

// TUIConfig defines the configuration for the Terminal User Interface.
type TUIConfig struct {
	Theme string `json:"theme,omitempty" mapstructure:"theme"`
}

// ShellConfig defines the configuration for the shell used by the bash tool.
type ShellConfig struct {
	Path string   `json:"path,omitempty" mapstructure:"path"`
	Args []string `json:"args,omitempty" mapstructure:"args"`
}

// Config is the main configuration structure for the application.
type Config struct {
	Data         Data                              `json:"data" mapstructure:"data"`
	WorkingDir   string                            `json:"wd,omitempty" mapstructure:"wd"`
	MCPServers   map[string]MCPServer              `json:"mcpServers,omitempty" mapstructure:"mcpservers"`
	Providers    map[models.ModelProvider]Provider `json:"providers,omitempty" mapstructure:"providers"`
	Provider     map[string]DynamicProviderDef     `json:"provider,omitempty" mapstructure:"provider"`
	Model        string                            `json:"model,omitempty" mapstructure:"model"`
	SmallModel   string                            `json:"small_model,omitempty" mapstructure:"small_model"`
	LSP          map[string]LSPConfig              `json:"lsp,omitempty" mapstructure:"lsp"`
	Debug        bool                              `json:"debug,omitempty" mapstructure:"debug"`
	DebugLSP     bool                              `json:"debugLSP,omitempty" mapstructure:"debuglsp"`
	ContextPaths []string                          `json:"contextPaths,omitempty" mapstructure:"contextpaths"`
	TUI          TUIConfig                         `json:"tui" mapstructure:"tui"`
	Shell        ShellConfig                       `json:"shell,omitempty" mapstructure:"shell"`
	AutoCompact  bool                              `json:"autoCompact,omitempty" mapstructure:"autocompact"`
}

// Application constants
const (
	defaultDataDirectory = ".opencode"
	defaultLogLevel      = "info"
	appName              = "opencode"

	MaxTokensFallbackDefault = 4096
)

var defaultContextPaths = []string{
	".github/copilot-instructions.md",
	".cursorrules",
	".cursor/rules/",
	"CLAUDE.md",
	"CLAUDE.local.md",
	"opencode.md",
	"opencode.local.md",
	"OpenCode.md",
	"OpenCode.local.md",
	"OPENCODE.md",
	"OPENCODE.local.md",
}

// Global configuration instance
var cfg *Config

// Load initializes the configuration from environment variables and config files.
// If debug is true, debug mode is enabled and log level is set to debug.
// It returns an error if configuration loading fails.
func Load(workingDir string, debug bool) (*Config, error) {
	if cfg != nil {
		return cfg, nil
	}

	cfg = &Config{
		WorkingDir: workingDir,
	}

	configureViper()
	setDefaults(debug)

	// First load local (project) config as the base
	mergeLocalConfig(workingDir)

	// Then read and merge global (home) config on top so it takes priority
	if err := readConfig(viper.MergeInConfig()); err != nil {
		return cfg, err
	}

	setProviderDefaults()

	// Apply configuration to the struct
	if err := viper.Unmarshal(cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Register dynamic providers from new-style "provider" config format
	registerDynamicProviders()

	applyDefaultValues()
	defaultLevel := slog.LevelInfo
	if cfg.Debug {
		defaultLevel = slog.LevelDebug
	}
	if os.Getenv("OPENCODE_DEV_DEBUG") == "true" {
		loggingFile := fmt.Sprintf("%s/%s", cfg.Data.Directory, "debug.log")
		messagesPath := fmt.Sprintf("%s/%s", cfg.Data.Directory, "messages")

		// if file does not exist create it
		if _, err := os.Stat(loggingFile); os.IsNotExist(err) {
			if err := os.MkdirAll(cfg.Data.Directory, 0o755); err != nil {
				return cfg, fmt.Errorf("failed to create directory: %w", err)
			}
			if _, err := os.Create(loggingFile); err != nil {
				return cfg, fmt.Errorf("failed to create log file: %w", err)
			}
		}

		if _, err := os.Stat(messagesPath); os.IsNotExist(err) {
			if err := os.MkdirAll(messagesPath, 0o756); err != nil {
				return cfg, fmt.Errorf("failed to create directory: %w", err)
			}
		}
		logging.MessageDir = messagesPath
		logging.InitPersistLog(messagesPath)

		sloggingFileWriter, err := os.OpenFile(loggingFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			return cfg, fmt.Errorf("failed to open log file: %w", err)
		}
		// Configure logger
		logger := slog.New(slog.NewTextHandler(sloggingFileWriter, &slog.HandlerOptions{
			Level: defaultLevel,
		}))
		slog.SetDefault(logger)
	} else {
		// Configure logger
		logger := slog.New(slog.NewTextHandler(logging.NewWriter(), &slog.HandlerOptions{
			Level: defaultLevel,
		}))
		slog.SetDefault(logger)
	}

	// Validate configuration
	if err := Validate(); err != nil {
		return cfg, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// configureViper sets up viper's configuration paths and environment variables.
func configureViper() {
	viper.SetConfigName(fmt.Sprintf(".%s", appName))
	viper.SetConfigType("json")
	viper.AddConfigPath("$HOME")
	viper.AddConfigPath(fmt.Sprintf("$XDG_CONFIG_HOME/%s", appName))
	viper.AddConfigPath(fmt.Sprintf("$HOME/.config/%s", appName))
	viper.SetEnvPrefix(strings.ToUpper(appName))
	viper.AutomaticEnv()
}

// setDefaults configures default values for configuration options.
func setDefaults(debug bool) {
	viper.SetDefault("data.directory", defaultDataDirectory)
	viper.SetDefault("contextPaths", defaultContextPaths)
	viper.SetDefault("tui.theme", "opencode")
	viper.SetDefault("autoCompact", true)

	// Set default shell from environment or fallback to /bin/bash
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/bash"
	}
	viper.SetDefault("shell.path", shellPath)
	viper.SetDefault("shell.args", []string{"-l"})

	if debug {
		viper.SetDefault("debug", true)
		viper.Set("log.level", "debug")
	} else {
		viper.SetDefault("debug", false)
		viper.SetDefault("log.level", defaultLogLevel)
	}
}

// setProviderDefaults configures LLM provider defaults based on provider provided by
// environment variables and configuration file.
func setProviderDefaults() {
	// Set all API keys we can find in the environment
	// Note: Viper does not default if the json apiKey is ""
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.anthropic.apiKey", apiKey)
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.openai.apiKey", apiKey)
	}
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.gemini.apiKey", apiKey)
	}
	if apiKey := os.Getenv("GROQ_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.groq.apiKey", apiKey)
	}
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.openrouter.apiKey", apiKey)
	}
	if apiKey := os.Getenv("ZHIPU_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.zhipu.apiKey", apiKey)
	}
	if apiKey := os.Getenv("XAI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.xai.apiKey", apiKey)
	}
	if apiKey := os.Getenv("AZURE_OPENAI_ENDPOINT"); apiKey != "" {
		// api-key may be empty when using Entra ID credentials – that's okay
		viper.SetDefault("providers.azure.apiKey", os.Getenv("AZURE_OPENAI_API_KEY"))
	}
	// NOTE: Copilot provider is NOT auto-registered here.
	// Users who want to use Copilot should explicitly configure it
	// in their opencode.json file. This prevents 401 errors when
	// a GitHub token exists but lacks Copilot subscription.

}

// hasAWSCredentials checks if AWS credentials are available in the environment.
func hasAWSCredentials() bool {
	// Check for explicit AWS credentials
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return true
	}

	// Check for AWS profile
	if os.Getenv("AWS_PROFILE") != "" || os.Getenv("AWS_DEFAULT_PROFILE") != "" {
		return true
	}

	// Check for AWS region
	if os.Getenv("AWS_REGION") != "" || os.Getenv("AWS_DEFAULT_REGION") != "" {
		return true
	}

	// Check if running on EC2 with instance profile
	if os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" ||
		os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" {
		return true
	}

	return false
}

// hasVertexAICredentials checks if VertexAI credentials are available in the environment.
func hasVertexAICredentials() bool {
	// Check for explicit VertexAI parameters
	if os.Getenv("VERTEXAI_PROJECT") != "" && os.Getenv("VERTEXAI_LOCATION") != "" {
		return true
	}
	// Check for Google Cloud project and location
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" && (os.Getenv("GOOGLE_CLOUD_REGION") != "" || os.Getenv("GOOGLE_CLOUD_LOCATION") != "") {
		return true
	}
	return false
}

// readConfig handles the result of reading a configuration file.
func readConfig(err error) error {
	if err == nil {
		return nil
	}

	// It's okay if the config file doesn't exist
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return nil
	}

	return fmt.Errorf("failed to read config: %w", err)
}

// mergeLocalConfig loads and merges configuration from the local directory.
// The local config is loaded as a base; global (home) config is merged on top with higher priority.
func mergeLocalConfig(workingDir string) {
	local := viper.New()
	local.SetConfigName(fmt.Sprintf(".%s", appName))
	local.SetConfigType("json")
	local.AddConfigPath(workingDir)

	// Load local config as base if it exists
	if err := local.ReadInConfig(); err == nil {
		viper.MergeConfigMap(local.AllSettings())
	}
}

func applyDefaultValues() {

	// Initialize maps if nil (Viper may not create them if config section is absent)
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]MCPServer)
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[models.ModelProvider]Provider)
	}
	if cfg.LSP == nil {
		cfg.LSP = make(map[string]LSPConfig)
	}

	logging.Info("MCP servers loaded from config", "count", len(cfg.MCPServers))
	for name, srv := range cfg.MCPServers {
		logging.Info("MCP server found", "name", name, "type", string(srv.Type), "url", srv.URL)
	}

	// Set default MCP type if not specified
	for k, v := range cfg.MCPServers {
		if v.Type == "" {
			v.Type = MCPStdio
			cfg.MCPServers[k] = v
		} else if v.Type == "http" {
			// Map logical http type to SSE protocol underneath
			v.Type = MCPSse
			cfg.MCPServers[k] = v
		}
	}
}

// Validate checks if the configuration is valid and applies defaults where needed.
func Validate() error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Validate providers
	for _, providerCfg := range cfg.Providers {
		if providerCfg.APIKey == "" && !providerCfg.Disabled {
		}
	}
	return nil
}

func GetConfigDir() string {
	return cfg.Data.Directory
}

// GetProvider gets the configuration for a provider.
func GetProvider(provider models.ModelProvider) (Provider, bool) {
	if cfg != nil {
		if p, ok := cfg.Providers[provider]; ok {
			return p, true
		}
	}
	return Provider{}, false
}

func updateCfgFile(updateCfg func(config *Config)) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Get the config file path
	configFile := viper.ConfigFileUsed()
	var configData []byte
	if configFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configFile = filepath.Join(homeDir, fmt.Sprintf(".%s.json", appName))
		logging.Info("config file not found, creating new one", "path", configFile)
		configData = []byte(`{}`)
	} else {
		// Read the existing config file
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		configData = data
	}

	// Parse the JSON
	var userCfg *Config
	if err := json.Unmarshal(configData, &userCfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	updateCfg(userCfg)

	// Write the updated config back to file
	updatedData, err := json.MarshalIndent(userCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, updatedData, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Get returns the current configuration.
// It's safe to call this function multiple times.
func Get() *Config {
	return cfg
}

// WorkingDirectory returns the current working directory from the configuration.
func WorkingDirectory() string {
	return cfg.WorkingDir
}

// ActiveModel returns the selected model overriding for all agents.
// The db object is fetched via callback to prevent import cycles.
var DbGetSetting func(context.Context, string) (string, error)
var DbSetSetting func(context.Context, string, string, int64, int64) error

func ActiveModel(ctx context.Context, defaultFallback models.ModelID) models.ModelID {
	// Collect all available models from enabled providers first
	var availableModels []string
	if cfg != nil {
		for _, m := range models.SupportedModels {
			if pCfg, pOk := cfg.Providers[m.Provider]; pOk && !pCfg.Disabled {
				availableModels = append(availableModels, string(m.ID))
			}
		}
		sort.Strings(availableModels)
	}

	// Helper function to validate if a model is available
	isModelAvailable := func(modelID models.ModelID) bool {
		model, ok := models.SupportedModels[modelID]
		if !ok {
			return false
		}
		if cfg == nil {
			return false
		}
		pCfg, pOk := cfg.Providers[model.Provider]
		return pOk && !pCfg.Disabled
	}

	// Try to get from database first
	if DbGetSetting != nil {
		value, err := DbGetSetting(ctx, "active_model")
		if err == nil && value != "" {
			dbModelID := models.ModelID(value)
			if isModelAvailable(dbModelID) {
				return dbModelID
			}
			// Database model is no longer available, clear it and fall through
			logging.Warn("Database model no longer available, selecting alternative", "model", value)
		}
	}

	// Use config file model if set
	if cfg != nil && cfg.Model != "" {
		configModelID := models.ModelID(cfg.Model)
		if isModelAvailable(configModelID) {
			return configModelID
		}
	}

	// Use default fallback if available
	if isModelAvailable(defaultFallback) {
		return defaultFallback
	}

	// Pick the first available model deterministically
	if len(availableModels) > 0 {
		selectedModel := models.ModelID(availableModels[0])
		// Auto-save the selected model to database for future use
		if DbSetSetting != nil {
			now := time.Now().UnixMilli()
			_ = DbSetSetting(ctx, "active_model", string(selectedModel), now, now)
		}
		logging.Info("Auto-selected first available model", "model", selectedModel)
		return selectedModel
	}

	// No models available at all
	return defaultFallback
}

// SaveActiveModel saves the globally selected model string key.
func SaveActiveModel(ctx context.Context, modelID models.ModelID) error {
	if DbSetSetting == nil {
		return fmt.Errorf("database not initialized")
	}

	_, ok := models.SupportedModels[modelID]
	if !ok {
		return fmt.Errorf("model %s not supported", modelID)
	}

	now := time.Now().UnixMilli()
	return DbSetSetting(ctx, "active_model", string(modelID), now, now)
}

// UpdateTheme updates the theme in the configuration and writes it to the config file.
func UpdateTheme(themeName string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Update the in-memory config
	cfg.TUI.Theme = themeName

	// Update the file config
	return updateCfgFile(func(config *Config) {
		config.TUI.Theme = themeName
	})
}

// Tries to load Github token from all possible locations
func LoadGitHubToken() (string, error) {
	// First check environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// Get config directory
	var configDir string
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		configDir = xdgConfig
	} else if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			configDir = localAppData
		} else {
			configDir = filepath.Join(os.Getenv("HOME"), "AppData", "Local")
		}
	} else {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}

	// Try both hosts.json and apps.json files
	filePaths := []string{
		filepath.Join(configDir, "github-copilot", "hosts.json"),
		filepath.Join(configDir, "github-copilot", "apps.json"),
	}

	for _, filePath := range filePaths {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var config map[string]map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}

		for key, value := range config {
			if strings.Contains(key, "github.com") {
				if oauthToken, ok := value["oauth_token"].(string); ok {
					return oauthToken, nil
				}
			}
		}
	}

	return "", fmt.Errorf("GitHub token not found in standard locations")
}

// registerDynamicProviders processes the new-style "provider" config field.
// For each entry with npm = "@ai-sdk/openai-compatible", it registers models
// into SupportedModels and injects provider credentials into cfg.Providers.
func registerDynamicProviders() {
	if cfg == nil || len(cfg.Provider) == 0 {
		return
	}
	for providerKey, def := range cfg.Provider {
		// Only support OpenAI-compatible providers or empty NPM for custom standard providers
		if def.NPM != "@ai-sdk/openai-compatible" && def.NPM != "" {
			continue
		}

		// Resolve API key: prefer top-level apiKey, fall back to options.apiKey
		apiKey := def.APIKey
		if apiKey == "" {
			apiKey = def.Options.APIKey
		}

		// Resolve base URL
		baseURL := def.Options.BaseURL

		logging.Info("Registering dynamic provider", "key", providerKey, "baseURL", baseURL, "apiKey", apiKey[:min(8, len(apiKey))])

		// Register all models declared in this provider
		for modelKey, modelCfg := range def.Models {
			name := modelCfg.Name
			if name == "" {
				name = modelKey
			}
			models.RegisterDynamicModel(providerKey, modelKey, name)
			logging.Info("Registered dynamic model", "model", providerKey+"."+modelKey, "name", name)
		}

		// Inject into the classic Providers map so provider routing works
		providerID := models.ModelProvider(providerKey)
		existing, exists := cfg.Providers[providerID]
		if !exists {
			cfg.Providers[providerID] = Provider{
				APIKey:  apiKey,
				BaseURL: baseURL,
			}
		} else {
			// Merge: prioritize values from provider config over environment defaults
			// Environment defaults are set in setProviderDefaults() and only contain API keys
			// The provider config should take precedence for both APIKey and BaseURL
			if apiKey != "" {
				existing.APIKey = apiKey
			}
			if baseURL != "" {
				existing.BaseURL = baseURL
			}
			cfg.Providers[providerID] = existing
		}
		logging.Info("Provider config", "providerID", providerID, "baseURL", cfg.Providers[providerID].BaseURL, "hasKey", cfg.Providers[providerID].APIKey != "")
	}
}

// ApplyTopLevelModelOverride maps the top-level "model" and "small_model" config fields
// to the agents configuration, if the agents don't already have explicit models.
// It is also called when the --model CLI flag is used to override the session model.
