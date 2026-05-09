package internal_custom_llm_common

import (
	"context"
	"net/http"
	"time"

	openai "github.com/openai/openai-go/v3"

	internal_callers "github.com/rapidaai/api/integration-api/internal/type"
	"github.com/rapidaai/pkg/commons"
	"github.com/rapidaai/protos"
)

type ClientConfig struct {
	Compatibility Compatibility
	BaseURL       string
	Headers       map[string]string

	ClientTimeout  time.Duration
	RequestTimeout time.Duration
}

type AdapterDependencies struct {
	Logger commons.Logger
	Config ClientConfig

	OpenAIClient *openai.Client
	HTTPClient   *http.Client
}

type Adapter interface {
	GetChatCompletion(
		ctx context.Context,
		allMessages []*protos.Message,
		options *internal_callers.ChatCompletionOptions,
	) (*protos.Message, []*protos.Metric, error)

	StreamChatCompletion(
		ctx context.Context,
		allMessages []*protos.Message,
		options *internal_callers.ChatCompletionOptions,
		onStream func(rID string, msg *protos.Message) error,
		onMetrics func(rID string, msg *protos.Message, mtrx []*protos.Metric) error,
		onError func(rID string, err error),
	) error

	VerifyCredential(
		ctx context.Context,
		options *internal_callers.CredentialVerifierOptions,
	) (*string, error)
}
