package kimi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultCodingModelsAreKimiNotClaude(t *testing.T) {
	t.Parallel()
	models := DefaultModels(true)
	require.NotEmpty(t, models)
	require.Equal(t, "kimi-k2.6", models[0].ID)
	for _, model := range models {
		require.NotContains(t, strings.ToLower(model.ID), "claude")
		require.True(t, strings.HasPrefix(model.ID, "kimi-"))
	}
}

func TestDefaultPayGModelsIncludeMoonshot(t *testing.T) {
	t.Parallel()
	ids := make([]string, 0, len(DefaultPayGModels()))
	for _, model := range DefaultModels(false) {
		ids = append(ids, model.ID)
		require.NotContains(t, strings.ToLower(model.ID), "claude")
	}
	require.Contains(t, ids, "kimi-k2")
	require.Contains(t, ids, "moonshot-v1-128k")
}
