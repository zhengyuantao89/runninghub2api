package httpapi

import "testing"

func TestNormalizePublicModelID(t *testing.T) {
	if normalizePublicModelID("nano_banana2") != ModelNanoBanana2LowCost {
		t.Fatal(normalizePublicModelID("nano_banana2"))
	}
	if normalizePublicModelID("gpt-image-2") != ModelGPTImage2LowCost {
		t.Fatal(normalizePublicModelID("gpt-image-2"))
	}
	if normalizePublicModelID("minimax-h3-rh-enhanced-multimodal") != ModelMinimaxH3RHEnhancedMultimodal {
		t.Fatal("multimodal")
	}
	if !isPublicImageModel(ModelNanoBanana2LowCost) || isPublicVideoModel(ModelNanoBanana2LowCost) {
		t.Fatal("image kind")
	}
	if !isPublicVideoModel(ModelMinimaxH3RHEnhanced) || isPublicImageModel(ModelMinimaxH3RHEnhanced) {
		t.Fatal("video kind")
	}
	if publicVideoCreationMode(ModelMinimaxH3RHEnhancedMultimodal) != "multimodal" {
		t.Fatal(publicVideoCreationMode(ModelMinimaxH3RHEnhancedMultimodal))
	}
}
