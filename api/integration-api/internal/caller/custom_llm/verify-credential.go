package internal_custom_llm_callers

import (
	"context"

	internal_callers "github.com/rapidaai/api/integration-api/internal/type"
	"github.com/rapidaai/pkg/commons"
	"github.com/rapidaai/protos"
)

type verifyCredentialCaller struct {
	*CustomLLM
}

func NewVerifyCredentialCaller(
	logger commons.Logger,
	credential *protos.Credential,
) internal_callers.Verifier {
	customLLM, err := New(logger, credential)
	if err != nil {
		logger.Errorf("custom-llm: failed to create verify credential caller: %v", err)
		customLLM = &CustomLLM{}
	}
	return &verifyCredentialCaller{
		CustomLLM: customLLM,
	}
}

func (vc *verifyCredentialCaller) CredentialVerifier(
	ctx context.Context,
	options *internal_callers.CredentialVerifierOptions,
) (*string, error) {
	adapter, err := vc.GetAdapter()
	if err != nil {
		return nil, err
	}
	return adapter.VerifyCredential(ctx, options)
}
