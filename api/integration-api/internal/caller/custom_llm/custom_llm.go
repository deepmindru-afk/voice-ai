// Rapida – Open Source Voice AI Orchestration Platform
// Copyright (C) 2023-2025 Prashant Srivastav <prashant@rapida.ai>
// Licensed under a modified GPL-2.0. See the LICENSE file for details.
package internal_custom_llm_callers

import (
	"fmt"

	internal_anthropic_messages "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/anthropic_messages"
	internal_custom_llm_common "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/common"
	internal_gemini_generate_content "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/gemini_generate_content"
	internal_openai_chat_completions "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/openai_chat_completions"
	internal_openai_compatible "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/openai_compatible"
	internal_openai_responses "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/openai_responses"
	"github.com/rapidaai/pkg/commons"
	integration_api "github.com/rapidaai/protos"
)

type CustomLLM struct {
	compatibility internal_custom_llm_common.Compatibility
	adapter       internal_custom_llm_common.Adapter
}

func New(
	logger commons.Logger,
	credential *integration_api.Credential,
) (*CustomLLM, error) {
	config, err := internal_custom_llm_common.ParseClientConfig(logger, credential)
	if err != nil {
		logger.Errorf("custom-llm: failed to parse credential config: %v", err)
		return nil, err
	}

	openAIClient := internal_custom_llm_common.NewOpenAIClient(config)
	dependencies := internal_custom_llm_common.AdapterDependencies{
		Logger:       logger,
		Config:       config,
		OpenAIClient: &openAIClient,
		HTTPClient:   internal_custom_llm_common.NewHTTPClient(config),
	}
	adapter, err := newAdapter(config.Compatibility, dependencies)
	if err != nil {
		logger.Errorf("custom-llm: failed to initialize compatibility adapter %q: %v", config.Compatibility, err)
		return nil, err
	}

	return &CustomLLM{
		compatibility: config.Compatibility,
		adapter:       adapter,
	}, nil
}

func (cl *CustomLLM) GetAdapter() (internal_custom_llm_common.Adapter, error) {
	if cl.adapter == nil {
		return nil, fmt.Errorf("custom-llm: adapter not initialized for compatibility %q", cl.compatibility)
	}
	return cl.adapter, nil
}

func newAdapter(
	compatibility internal_custom_llm_common.Compatibility,
	dependencies internal_custom_llm_common.AdapterDependencies,
) (internal_custom_llm_common.Adapter, error) {
	switch compatibility {
	case internal_custom_llm_common.CompatibilityOpenAIChatCompletions:
		return internal_openai_chat_completions.New(dependencies)
	case internal_custom_llm_common.CompatibilityOpenAIResponses:
		return internal_openai_responses.New(dependencies)
	case internal_custom_llm_common.CompatibilityOpenAICompatible:
		return internal_openai_compatible.New(dependencies)
	case internal_custom_llm_common.CompatibilityAnthropicMessages:
		return internal_anthropic_messages.New(dependencies)
	case internal_custom_llm_common.CompatibilityGeminiGenerateContent:
		return internal_gemini_generate_content.New(dependencies)
	default:
		return nil, internal_custom_llm_common.UnsupportedCompatibilityError{
			Compatibility: compatibility,
		}
	}
}
