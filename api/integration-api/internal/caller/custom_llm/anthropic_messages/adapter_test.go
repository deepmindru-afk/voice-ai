package internal_custom_llm_anthropic_messages

import (
	"context"
	"testing"

	internal_custom_llm_common "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/common"
	internal_callers "github.com/rapidaai/api/integration-api/internal/type"
	"github.com/rapidaai/protos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapter_ReturnsNotImplementedErrors(t *testing.T) {
	ad := &adapter{}

	_, _, err := ad.GetChatCompletion(context.Background(), nil, &internal_callers.ChatCompletionOptions{
		Request: &protos.ChatRequest{RequestId: "req-id"},
	})
	require.Error(t, err)
	_, ok := err.(internal_custom_llm_common.NotImplementedCompatibilityError)
	assert.True(t, ok)

	err = ad.StreamChatCompletion(context.Background(), nil, &internal_callers.ChatCompletionOptions{
		Request: &protos.ChatRequest{RequestId: "req-id"},
	}, nil, nil, nil)
	require.Error(t, err)
	_, ok = err.(internal_custom_llm_common.NotImplementedCompatibilityError)
	assert.True(t, ok)

	_, err = ad.VerifyCredential(context.Background(), &internal_callers.CredentialVerifierOptions{})
	require.Error(t, err)
	_, ok = err.(internal_custom_llm_common.NotImplementedCompatibilityError)
	assert.True(t, ok)
}
