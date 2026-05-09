package internal_custom_llm_callers

import (
	"context"
	"testing"

	internal_custom_llm_common "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/common"
	internal_callers "github.com/rapidaai/api/integration-api/internal/type"
	"github.com/rapidaai/pkg/commons"
	"github.com/rapidaai/protos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func newTestLogger() commons.Logger {
	logger, _ := commons.NewApplicationLogger()
	return logger
}

func newCredential(
	t *testing.T,
	values map[string]interface{},
) *protos.Credential {
	t.Helper()
	pb, err := structpb.NewStruct(values)
	require.NoError(t, err)
	return &protos.Credential{
		Value: pb,
	}
}

func TestNewLargeLanguageCaller_RoutesAllCompatibilities(t *testing.T) {
	tests := []struct {
		name          string
		compatibility string
		want          internal_custom_llm_common.Compatibility
	}{
		{
			name:          "openai chat completions",
			compatibility: "openai_chat_completions",
			want:          internal_custom_llm_common.CompatibilityOpenAIChatCompletions,
		},
		{
			name:          "openai responses",
			compatibility: "openai_responses",
			want:          internal_custom_llm_common.CompatibilityOpenAIResponses,
		},
		{
			name:          "openai compatible",
			compatibility: "openai_compatible",
			want:          internal_custom_llm_common.CompatibilityOpenAICompatible,
		},
		{
			name:          "anthropic messages",
			compatibility: "anthropic_messages",
			want:          internal_custom_llm_common.CompatibilityAnthropicMessages,
		},
		{
			name:          "gemini generate content",
			compatibility: "gemini_generate_content",
			want:          internal_custom_llm_common.CompatibilityGeminiGenerateContent,
		},
		{
			name:          "legacy openai maps to chat completions",
			compatibility: "openai",
			want:          internal_custom_llm_common.CompatibilityOpenAIChatCompletions,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller, err := NewLargeLanguageCaller(newTestLogger(), newCredential(t, map[string]interface{}{
				"api_compatibility": tc.compatibility,
				"base_url":          "http://localhost:8000/v1",
			}))
			require.NoError(t, err)
			llc, ok := caller.(*largeLanguageCaller)
			require.True(t, ok)
			assert.Equal(t, tc.want, llc.compatibility)
		})
	}
}

func TestNewLargeLanguageCaller_UsesDefaultsAndCamelCaseKeys(t *testing.T) {
	t.Run("defaults compatibility", func(t *testing.T) {
		caller, err := NewLargeLanguageCaller(newTestLogger(), newCredential(t, map[string]interface{}{
			"base_url": "http://localhost:8000/v1",
		}))
		require.NoError(t, err)
		llc := caller.(*largeLanguageCaller)
		assert.Equal(t, internal_custom_llm_common.CompatibilityOpenAIChatCompletions, llc.compatibility)
	})

	t.Run("camelCase compatibility and base url", func(t *testing.T) {
		caller, err := NewLargeLanguageCaller(newTestLogger(), newCredential(t, map[string]interface{}{
			"apiCompatibility": "openai_responses",
			"baseUrl":          "http://localhost:8000/v1",
		}))
		require.NoError(t, err)
		llc := caller.(*largeLanguageCaller)
		assert.Equal(t, internal_custom_llm_common.CompatibilityOpenAIResponses, llc.compatibility)
	})
}

func TestNewLargeLanguageCaller_RejectsUnsupportedCompatibility(t *testing.T) {
	_, err := NewLargeLanguageCaller(newTestLogger(), newCredential(t, map[string]interface{}{
		"api_compatibility": "unsupported_api",
		"base_url":          "http://localhost:8000/v1",
	}))
	require.Error(t, err)
	_, ok := err.(internal_custom_llm_common.UnsupportedCompatibilityError)
	assert.True(t, ok)
}

func TestNotImplementedAdapters_ReturnDeterministicErrors(t *testing.T) {
	tests := []string{
		"anthropic_messages",
		"gemini_generate_content",
	}
	for _, compatibility := range tests {
		t.Run(compatibility, func(t *testing.T) {
			caller, err := NewLargeLanguageCaller(newTestLogger(), newCredential(t, map[string]interface{}{
				"api_compatibility": compatibility,
				"base_url":          "http://localhost:8000/v1",
			}))
			require.NoError(t, err)

			_, _, err = caller.GetChatCompletion(
				context.Background(),
				nil,
				&internal_callers.ChatCompletionOptions{
					Request: &protos.ChatRequest{RequestId: "req-id"},
				},
			)
			require.Error(t, err)
			_, ok := err.(internal_custom_llm_common.NotImplementedCompatibilityError)
			assert.True(t, ok)
		})
	}
}
