package models

const (
	ProviderZhipu ModelProvider = "zhipu"
)

const (
	ZhipuGLM47    ModelID = "zhipu.glm-4.7"
	ZhipuGLM4Plus ModelID = "zhipu.glm-4-plus"
	ZhipuGLM4Air  ModelID = "zhipu.glm-4-air"
)

var ZhipuModels = map[ModelID]Model{
	ZhipuGLM47: {
		ID:                  ZhipuGLM47,
		Name:                "GLM-4.7",
		Provider:            ProviderZhipu,
		APIModel:            "glm-4.7",
		CostPer1MIn:         0,
		CostPer1MOut:        0,
		ContextWindow:       128000,
		DefaultMaxTokens:    4096,
		CanReason:           false,
		SupportsAttachments: false,
	},
	ZhipuGLM4Plus: {
		ID:                  ZhipuGLM4Plus,
		Name:                "GLM-4-Plus",
		Provider:            ProviderZhipu,
		APIModel:            "glm-4-plus",
		CostPer1MIn:         0,
		CostPer1MOut:        0,
		ContextWindow:       128000,
		DefaultMaxTokens:    4096,
		CanReason:           false,
		SupportsAttachments: false,
	},
	ZhipuGLM4Air: {
		ID:                  ZhipuGLM4Air,
		Name:                "GLM-4-Air",
		Provider:            ProviderZhipu,
		APIModel:            "glm-4-air",
		CostPer1MIn:         0,
		CostPer1MOut:        0,
		ContextWindow:       128000,
		DefaultMaxTokens:    4096,
		CanReason:           false,
		SupportsAttachments: false,
	},
}
