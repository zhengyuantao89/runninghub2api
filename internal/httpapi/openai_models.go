package httpapi

import (
	"strings"

	"github.com/trae/runninghub2api/internal/provider/runninghub"
)

// Public OpenAI-compatible model IDs exposed to downstream callers.
// Callers pass these in JSON field "model" — never submit_mode.
const (
	ModelNanoBanana2LowCost            = "nano-banana2-low-cost"
	ModelGPTImage2LowCost              = "gpt-image-2-low-cost"
	ModelMinimaxH3RHEnhanced           = "minimax-h3-rh-enhanced"
	ModelMinimaxH3RHEnhancedMultimodal = "minimax-h3-rh-enhanced-multimodal"
)

type openAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
	// Extensions for our console / docs.
	Kind         string `json:"kind"` // image | video
	DisplayName  string `json:"display_name"`
	DefaultSync  bool   `json:"default_sync"`
	MaxRefs      int    `json:"max_refs,omitempty"`
	CreationMode string `json:"creation_mode,omitempty"` // video only
}

func openAIModelCatalog() []openAIModel {
	const created = 1762700000 // stable catalog epoch
	return []openAIModel{
		{
			ID: ModelNanoBanana2LowCost, Object: "model", Created: created, OwnedBy: "runninghub",
			Kind: "image", DisplayName: runninghub.DefaultQCFreeImageName, DefaultSync: true, MaxRefs: runninghub.DefaultQCFreeImageMaxRefs,
		},
		{
			ID: ModelGPTImage2LowCost, Object: "model", Created: created, OwnedBy: "runninghub",
			Kind: "image", DisplayName: runninghub.DefaultQCFreeGPTImageName, DefaultSync: true, MaxRefs: runninghub.DefaultQCFreeImageMaxRefs,
		},
		{
			ID: ModelMinimaxH3RHEnhanced, Object: "model", Created: created, OwnedBy: "runninghub",
			Kind: "video", DisplayName: runninghub.DefaultQCFreeVideoName + " · 自由创作",
			DefaultSync: false, MaxRefs: 1, CreationMode: runninghub.QCFreeVideoModeFreeCreation,
		},
		{
			ID: ModelMinimaxH3RHEnhancedMultimodal, Object: "model", Created: created, OwnedBy: "runninghub",
			Kind: "video", DisplayName: runninghub.DefaultQCFreeVideoName + " · 多模态参考",
			DefaultSync: false, MaxRefs: runninghub.DefaultQCFreeVideoMaxRefs, CreationMode: runninghub.QCFreeVideoModeMultimodal,
		},
	}
}

func findOpenAIModel(id string) (openAIModel, bool) {
	id = normalizePublicModelID(id)
	for _, m := range openAIModelCatalog() {
		if m.ID == id {
			return m, true
		}
	}
	return openAIModel{}, false
}

// normalizePublicModelID maps aliases → canonical public model id.
func normalizePublicModelID(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	switch s {
	case "", "nano-banana2", "nano-banana-2", "nano-banana2-low-cost", "nanobanana2",
		runninghub.DefaultQCFreeImageFamilyNano, "nano-banana2-lowcost":
		return ModelNanoBanana2LowCost
	case "gpt-image-2", "gpt-image2", "gpt-image-2-low-cost", "gptimage2",
		runninghub.DefaultQCFreeImageFamilyGPT, "gpt-image-2.0-low-cost", "gpt-image2.0-low-cost":
		return ModelGPTImage2LowCost
	case "minimax-h3-rh-enhanced", "minimax-h3", "minimax-h3-rh", "minimax":
		return ModelMinimaxH3RHEnhanced
	case "minimax-h3-rh-enhanced-multimodal", "minimax-h3-multimodal", "minimax-multimodal":
		return ModelMinimaxH3RHEnhancedMultimodal
	default:
		return s
	}
}

func publicImageFamily(modelID string) string {
	switch normalizePublicModelID(modelID) {
	case ModelGPTImage2LowCost:
		return runninghub.DefaultQCFreeImageFamilyGPT
	default:
		return runninghub.DefaultQCFreeImageFamilyNano
	}
}

func publicVideoCreationMode(modelID string) string {
	switch normalizePublicModelID(modelID) {
	case ModelMinimaxH3RHEnhancedMultimodal:
		return runninghub.QCFreeVideoModeMultimodal
	default:
		return runninghub.QCFreeVideoModeFreeCreation
	}
}

func isPublicImageModel(modelID string) bool {
	m, ok := findOpenAIModel(modelID)
	return ok && m.Kind == "image"
}

func isPublicVideoModel(modelID string) bool {
	m, ok := findOpenAIModel(modelID)
	return ok && m.Kind == "video"
}
