package models

const (
	ProviderZhipu ModelProvider = "zhipu"
)

const (
	ZhipuGLM47    ModelID = "zhipu.glm-4.7"
	ZhipuGLM4Plus ModelID = "zhipu.glm-4-plus"
	ZhipuGLM40520 ModelID = "zhipu.glm-4-0520"
	ZhipuGLM4AirX ModelID = "zhipu.glm-4-airx"
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
		DefaultMaxTokens:    16384,
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
		DefaultMaxTokens:    16384,
		CanReason:           false,
		SupportsAttachments: false,
	},
	ZhipuGLM40520: {
		ID:                  ZhipuGLM40520,
		Name:                "GLM-4-0520",
		Provider:            ProviderZhipu,
		APIModel:            "glm-4-0520",
		CostPer1MIn:         0,
		CostPer1MOut:        0,
		ContextWindow:       128000,
		DefaultMaxTokens:    16384,
		CanReason:           false,
		SupportsAttachments: false,
	},
	ZhipuGLM4AirX: {
		ID:                  ZhipuGLM4AirX,
		Name:                "GLM-4-AirX",
		Provider:            ProviderZhipu,
		APIModel:            "glm-4-airx",
		CostPer1MIn:         0,
		CostPer1MOut:        0,
		ContextWindow:       8192,
		DefaultMaxTokens:    16384,
		CanReason:           false,
		SupportsAttachments: false,
	},
}
