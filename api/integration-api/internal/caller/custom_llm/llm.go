package internal_custom_llm_callers

import (
	"context"

	internal_callers "github.com/rapidaai/api/integration-api/internal/type"
	"github.com/rapidaai/pkg/commons"
	"github.com/rapidaai/protos"
)

type largeLanguageCaller struct {
	*CustomLLM
}

func NewLargeLanguageCaller(
	logger commons.Logger,
	credential *protos.Credential,
) (internal_callers.LargeLanguageCaller, error) {
	customLLM, err := New(logger, credential)
	if err != nil {
		logger.Errorf("custom-llm: failed to create large language caller: %v", err)
		return nil, err
	}
	return &largeLanguageCaller{
		CustomLLM: customLLM,
	}, nil
}

func (llc *largeLanguageCaller) GetChatCompletion(
	ctx context.Context,
	allMessages []*protos.Message,
	options *internal_callers.ChatCompletionOptions,
) (*protos.Message, []*protos.Metric, error) {
	adapter, err := llc.GetAdapter()
	if err != nil {
		return nil, nil, err
	}
	return adapter.GetChatCompletion(ctx, allMessages, options)
}

func (llc *largeLanguageCaller) StreamChatCompletion(
	ctx context.Context,
	allMessages []*protos.Message,
	options *internal_callers.ChatCompletionOptions,
	onStream func(string, *protos.Message) error,
	onMetrics func(string, *protos.Message, []*protos.Metric) error,
	onError func(string, error),
) error {
	adapter, err := llc.GetAdapter()
	if err != nil {
		return err
	}
	return adapter.StreamChatCompletion(
		ctx,
		allMessages,
		options,
		onStream,
		onMetrics,
		onError,
	)
}
