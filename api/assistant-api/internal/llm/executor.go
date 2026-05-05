// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package internal_llm

import (
	"context"
	"errors"

	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	"github.com/rapidaai/protos"

	internal_agentkit "github.com/rapidaai/api/assistant-api/internal/llm/internal/agentkit"
	internal_model "github.com/rapidaai/api/assistant-api/internal/llm/internal/model"
	internal_websocket "github.com/rapidaai/api/assistant-api/internal/llm/internal/websocket"
	"github.com/rapidaai/pkg/commons"
	type_enums "github.com/rapidaai/pkg/types/enums"
)

/*
AssistantExecutor and its related interfaces define the contract for executing
assistant-related actions in the system. These interfaces are crucial for
implementing various modes of interaction with the assistant, such as text-based
chat and voice communication.

AssistantMessageExecutor handles text-based chat interactions. It defines a Chat
method that processes messaging requests and returns any errors encountered during
the chat process.

AssistantTalkExecutor is responsible for voice-based interactions. Its Talk method
takes care of processing talking requests and handles any errors that may occur
during the voice interaction.

AssistantExecutor combines both text and voice capabilities, allowing for a more
versatile assistant that can handle multiple modes of communication. By embedding
both AssistantMessageExecutor and AssistantTalkExecutor, it ensures that any
implementing type can handle both chat and talk functionalities.

These interfaces provide a clean separation of concerns and allow for easy
extension of the assistant's capabilities in the future. They also promote
loose coupling between the assistant's implementation and the rest of the system,
making it easier to maintain and evolve the codebase over time.
*/

type AssistantExecutor interface {

	// Initialize sets up all fields after creation
	Initialize(ctx context.Context, communication internal_type.Communication, cfg *protos.ConversationInitialization) error

	// name
	Name() string

	// Execute processes an incoming packet
	Execute(ctx context.Context, communication internal_type.Communication, pctk internal_type.Packet) error

	// disconnect
	Close(ctx context.Context) error
}

type assistantExecutor struct {
	logger   commons.Logger
	executor AssistantExecutor
}

func NewAssistantExecutor(logger commons.Logger) AssistantExecutor {
	return &assistantExecutor{
		logger: logger,
	}
}

// Init implements internal_executors.AssistantExecutor.
func (a *assistantExecutor) Initialize(ctx context.Context, communication internal_type.Communication, cfg *protos.ConversationInitialization) error {
	switch communication.Assistant().AssistantProvider {
	case type_enums.AGENTKIT:
		a.executor = internal_agentkit.NewAgentKitAssistantExecutor(a.logger)
	case type_enums.WEBSOCKET:
		a.executor = internal_websocket.NewWebsocketAssistantExecutor(a.logger)
	case type_enums.MODEL:
		a.executor = internal_model.NewModelAssistantExecutor(a.logger)
	default:
		return errors.New("illegal assistant executor")
	}
	return a.executor.Initialize(ctx, communication, cfg)
}

// Name implements internal_executors.AssistantExecutor.
func (a *assistantExecutor) Name() string {
	return a.executor.Name()
}

// Talk implements internal_executors.AssistantExecutor.
func (a *assistantExecutor) Execute(ctx context.Context, communication internal_type.Communication, pctk internal_type.Packet) error {
	if a.executor == nil {
		return errors.New("assistant executor not initialized")
	}
	return a.executor.Execute(ctx, communication, pctk)
}

func (a *assistantExecutor) Close(ctx context.Context) error {
	if a.executor == nil {
		return errors.New("assistant executor not initialized")
	}
	return a.executor.Close(ctx)
}
