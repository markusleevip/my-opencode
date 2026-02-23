package models

import (
	"maps"
)

type (
	ModelID       string
	ModelProvider string
)

type Model struct {
	ID                  ModelID       `json:"id"`
	Name                string        `json:"name"`
	Provider            ModelProvider `json:"provider"`
	APIModel            string        `json:"api_model"`
	CostPer1MIn         float64       `json:"cost_per_1m_in"`
	CostPer1MOut        float64       `json:"cost_per_1m_out"`
	CostPer1MInCached   float64       `json:"cost_per_1m_in_cached"`
	CostPer1MOutCached  float64       `json:"cost_per_1m_out_cached"`
	ContextWindow       int64         `json:"context_window"`
	DefaultMaxTokens    int64         `json:"default_max_tokens"`
	CanReason           bool          `json:"can_reason"`
	SupportsAttachments bool          `json:"supports_attachments"`
}

// Model IDs
const (
	// Bedrock
	BedrockClaude37Sonnet ModelID = "bedrock.claude-3.7-sonnet"
)

const (
	ProviderBedrock ModelProvider = "bedrock"
	// ForTests
	ProviderMock ModelProvider = "__mock"
)

// Providers in order of popularity
var ProviderPopularity = map[ModelProvider]int{
	ProviderCopilot:    1,
	ProviderAnthropic:  2,
	ProviderOpenAI:     3,
	ProviderGemini:     4,
	ProviderGROQ:       5,
	ProviderOpenRouter: 6,
	ProviderBedrock:    7,
	ProviderAzure:      8,
	ProviderVertexAI:   9,
	ProviderZhipu:      10,
}

// SupportedModels maps model names to their configurations.
var SupportedModels = make(map[ModelID]Model)

// ConfigSetter is a callback set by the config package to avoid circular dependencies.
var ConfigSetter func(key string, value any)

func RegisterModel(id ModelID, model Model) {
	SupportedModels[id] = model
}

func init() {
	maps.Copy(SupportedModels, AnthropicModels)
	maps.Copy(SupportedModels, OpenAIModels)
	maps.Copy(SupportedModels, GeminiModels)
	maps.Copy(SupportedModels, GroqModels)
	maps.Copy(SupportedModels, AzureModels)
	maps.Copy(SupportedModels, OpenRouterModels)
	maps.Copy(SupportedModels, XAIModels)
	maps.Copy(SupportedModels, VertexAIGeminiModels)
	maps.Copy(SupportedModels, CopilotModels)
	maps.Copy(SupportedModels, ZhipuModels)

	// Add the BedrockClaude37Sonnet model directly as it was in the original SupportedModels map
	SupportedModels[BedrockClaude37Sonnet] = Model{
		ID:                 BedrockClaude37Sonnet,
		Name:               "Bedrock: Claude 3.7 Sonnet",
		Provider:           ProviderBedrock,
		APIModel:           "anthropic.claude-3-7-sonnet-20250219-v1:0",
		CostPer1MIn:        3.0,
		CostPer1MInCached:  3.75,
		CostPer1MOutCached: 0.30,
		CostPer1MOut:       15.0,
	}
}

// RegisterDynamicModel registers a model from a dynamic provider definition.
// The model ID will be "{providerKey}.{modelKey}" (e.g. "zhipu.glm-4.7").
// It is safe to call multiple times; existing registrations are overwritten.
func RegisterDynamicModel(providerKey, modelKey, apiModel, modelName string, maxTokens, contextWindow int64, canReason, supportsAttachments bool) ModelID {
	id := ModelID(providerKey + "::" + modelKey)

	// Fallback to defaults if not specified
	if contextWindow <= 0 {
		contextWindow = 128000
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	model := Model{
		ID:                  id,
		Name:                modelName,
		Provider:            ModelProvider(providerKey),
		APIModel:            apiModel,
		ContextWindow:       contextWindow,
		DefaultMaxTokens:    maxTokens,
		CanReason:           canReason,
		SupportsAttachments: supportsAttachments,
	}
	SupportedModels[id] = model
	return id
}
