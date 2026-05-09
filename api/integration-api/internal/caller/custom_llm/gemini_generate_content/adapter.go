package internal_custom_llm_gemini_generate_content

import (
	"context"

	internal_custom_llm_common "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/common"
	internal_callers "github.com/rapidaai/api/integration-api/internal/type"
	"github.com/rapidaai/protos"
)

type adapter struct{}

func New(
	_ internal_custom_llm_common.AdapterDependencies,
) (internal_custom_llm_common.Adapter, error) {
	return &adapter{}, nil
}

func (a *adapter) GetChatCompletion(
	ctx context.Context,
	allMessages []*protos.Message,
	options *internal_callers.ChatCompletionOptions,
) (*protos.Message, []*protos.Metric, error) {
	return nil, nil, internal_custom_llm_common.NotImplementedCompatibilityError{
		Compatibility: internal_custom_llm_common.CompatibilityGeminiGenerateContent,
		Feature:       "GetChatCompletion",
	}
}

func (a *adapter) StreamChatCompletion(
	ctx context.Context,
	allMessages []*protos.Message,
	options *internal_callers.ChatCompletionOptions,
	onStream func(string, *protos.Message) error,
	onMetrics func(string, *protos.Message, []*protos.Metric) error,
	onError func(string, error),
) error {
	err := internal_custom_llm_common.NotImplementedCompatibilityError{
		Compatibility: internal_custom_llm_common.CompatibilityGeminiGenerateContent,
		Feature:       "StreamChatCompletion",
	}
	if onError != nil && options != nil && options.Request != nil {
		onError(options.Request.GetRequestId(), err)
	}
	return err
}

func (a *adapter) VerifyCredential(
	ctx context.Context,
	options *internal_callers.CredentialVerifierOptions,
) (*string, error) {
	return nil, internal_custom_llm_common.NotImplementedCompatibilityError{
		Compatibility: internal_custom_llm_common.CompatibilityGeminiGenerateContent,
		Feature:       "VerifyCredential",
	}
}
