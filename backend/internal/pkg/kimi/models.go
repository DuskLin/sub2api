package kimi

// Model is the admin test-picker catalog entry for Kimi accounts.
type Model struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

var defaultCodingModels = []Model{
	{ID: "kimi-k2.6", Type: "model", DisplayName: "Kimi K2.6"},
	{ID: "kimi-k2.5", Type: "model", DisplayName: "Kimi K2.5"},
	{ID: "kimi-k2-thinking", Type: "model", DisplayName: "Kimi K2 Thinking"},
	{ID: "kimi-k2", Type: "model", DisplayName: "Kimi K2"},
	{ID: "kimi-for-coding", Type: "model", DisplayName: "Kimi for Coding"},
	{ID: "kimi-k3", Type: "model", DisplayName: "Kimi K3"},
}

var defaultPayGModels = []Model{
	{ID: "kimi-k2.5", Type: "model", DisplayName: "Kimi K2.5"},
	{ID: "kimi-k2", Type: "model", DisplayName: "Kimi K2"},
	{ID: "kimi-latest", Type: "model", DisplayName: "Kimi Latest"},
	{ID: "moonshot-v1-128k", Type: "model", DisplayName: "Moonshot v1 128k"},
	{ID: "moonshot-v1-32k", Type: "model", DisplayName: "Moonshot v1 32k"},
	{ID: "moonshot-v1-8k", Type: "model", DisplayName: "Moonshot v1 8k"},
}

// DefaultCodingModels is the Kimi Code / Coding Plan catalog used by OAuth
// accounts and coding-plan API keys when no model_mapping is configured.
func DefaultCodingModels() []Model {
	out := make([]Model, len(defaultCodingModels))
	copy(out, defaultCodingModels)
	return out
}

// DefaultPayGModels is the Moonshot pay-as-you-go catalog.
func DefaultPayGModels() []Model {
	out := make([]Model, len(defaultPayGModels))
	copy(out, defaultPayGModels)
	return out
}

// DefaultModels returns the Kimi test-picker catalog for coding or payg.
func DefaultModels(coding bool) []Model {
	if coding {
		return DefaultCodingModels()
	}
	return DefaultPayGModels()
}
