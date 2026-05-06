// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package adapter_internal

import (
	"context"
	"fmt"
	"time"

	observe "github.com/rapidaai/api/assistant-api/internal/observe"
	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	"github.com/rapidaai/pkg/types"
	type_enums "github.com/rapidaai/pkg/types/enums"
	"github.com/rapidaai/protos"
)

// =============================================================================
// Talk - Main Entry Point
// =============================================================================

// Talk handles the main conversation loop for different streamer types.
// It processes incoming messages and manages the connection lifecycle.
//
// Shutdown relies on Recv() returning an error (EOF or context-cancelled)
// or a ConversationDisconnection message. All streamer implementations
// guarantee one of these when the connection ends.
func (t *genericRequestor) Talk(_ context.Context, auth types.SimplePrinciple) error {
	totalTime := time.Now()
	for {
		req, err := t.streamer.Recv()
		if err != nil {
			if t.Conversation() != nil {
				t.emitCallCompletion(totalTime)
				t.Disconnect(context.Background())
			}
			return nil
		}
		if t.handleStreamInput(t.streamer.Context(), auth, totalTime, req) {
			return nil
		}
	}
}

func (t *genericRequestor) handleStreamInput(ctx context.Context, auth types.SimplePrinciple, totalTime time.Time, req internal_type.Stream) bool {
	switch payload := req.(type) {
	case *protos.ConversationInitialization:
		_ = t.Connect(ctx, auth, payload)
		t.streamer.NotifyMode(payload.GetStreamMode())
	case *protos.ConversationConfiguration:
		t.handleStreamModeSwitch(ctx, payload)
	case *protos.ConversationUserMessage:
		t.handleStreamUserMessage(ctx, payload)
	case *protos.ConversationToolCallResult:
		t.OnPacket(ctx, internal_type.LLMToolResultPacket{
			ToolID:    payload.GetToolId(),
			Name:      payload.GetName(),
			ContextID: payload.GetId(),
			Action:    payload.GetAction(),
			Result:    payload.GetResult(),
		})
	case *protos.ConversationBridgeUserAudio:
		t.OnPacket(ctx, internal_type.RecordUserAudioPacket{ContextID: t.GetID(), Audio: payload.Audio})
	case *protos.ConversationBridgeOperatorAudio:
		t.OnPacket(ctx, internal_type.RecordAssistantAudioPacket{ContextID: t.GetID(), Audio: payload.Audio})
	case *protos.ConversationMetadata:
		t.OnPacket(ctx, internal_type.ConversationMetadataPacket{
			ContextID: payload.GetAssistantConversationId(),
			Metadata:  payload.GetMetadata(),
		})
	case *protos.ConversationMetric:
		t.OnPacket(ctx, internal_type.ConversationMetricPacket{
			ContextID: payload.GetAssistantConversationId(),
			Metrics:   payload.GetMetrics(),
		})
	case *protos.ConversationEvent:
		eventTime := time.Now()
		if payload.Time != nil {
			eventTime = payload.Time.AsTime()
		}
		t.OnPacket(ctx, internal_type.ConversationEventPacket{
			Name: payload.Name,
			Data: payload.Data,
			Time: eventTime,
		})
	case *protos.ConversationDisconnection:
		if t.Conversation() == nil {
			return true
		}
		t.handleStreamDisconnection(totalTime, payload)
		return true
	}

	return false
}

func (t *genericRequestor) handleStreamModeSwitch(ctx context.Context, payload *protos.ConversationConfiguration) {
	t.OnPacket(ctx, internal_type.ModeSwitchRequestedPacket{
		ContextID:   t.GetID(),
		StreamMode:  payload.GetStreamMode(),
		RequestedAt: time.Now(),
	})
}

func (t *genericRequestor) handleStreamUserMessage(ctx context.Context, payload *protos.ConversationUserMessage) {
	switch msg := payload.GetMessage().(type) {
	case *protos.ConversationUserMessage_Audio:
		t.OnPacket(ctx, internal_type.UserAudioReceivedPacket{ContextID: t.GetID(), Audio: msg.Audio})
	case *protos.ConversationUserMessage_Text:
		t.OnPacket(ctx, internal_type.UserTextReceivedPacket{ContextID: t.GetID(), Text: msg.Text})
	default:
		t.logger.Errorf("illegal input from the user %+v", msg)
	}
}

func (t *genericRequestor) handleStreamDisconnection(totalTime time.Time, payload *protos.ConversationDisconnection) {
	ctx := context.Background()
	t.OnPacket(ctx,
		internal_type.ConversationEventPacket{
			ContextID: t.GetID(),
			Name:      observe.ComponentSession,
			Data:      map[string]string{observe.DataType: observe.EventDisconnectRequested, observe.DataReason: payload.GetType().String()},
			Time:      time.Now(),
		},
		internal_type.ConversationMetadataPacket{
			ContextID: t.Conversation().Id,
			Metadata: []*protos.Metadata{{
				Key:   "disconnect_reason",
				Value: payload.GetType().String(),
			}},
		})
	t.emitCallCompletion(totalTime)
	t.Disconnect(ctx)
}

// emitCallCompletion persists final metrics and events when the talk loop exits.
// Written directly with a background context because the dispatcher goroutine's
// context is already cancelled when Recv() returns an error.
func (t *genericRequestor) emitCallCompletion(startTime time.Time) {
	duration := time.Since(startTime)
	completionMetrics := []*protos.Metric{
		{
			Name:        type_enums.CONVERSATION_STATUS.String(),
			Value:       type_enums.CONVERSATION_COMPLETE.String(),
			Description: "Status of current conversation",
		},
		{
			Name:        type_enums.CONVERSATION_DURATION.String(),
			Value:       fmt.Sprintf("%d", duration),
			Description: "Conversation duration from first message to end",
		},
	}
	if err := t.onAddMetrics(context.Background(), completionMetrics...); err != nil {
		t.logger.Errorf("talk: failed to persist completion metrics: %v", err)
	}
	if t.observer != nil {
		t.observer.MetricCollectors().Collect(context.Background(), observe.ConversationMetricRecord{
			ConversationID: fmt.Sprintf("%d", t.Conversation().Id),
			Metrics:        completionMetrics,
			Time:           time.Now(),
		})
		t.observer.EventCollectors().Collect(context.Background(), observe.EventRecord{
			MessageID: t.GetID(),
			Name:      observe.ComponentSession,
			Data: map[string]string{
				observe.DataType:     observe.EventCompleted,
				observe.DataDuration: fmt.Sprintf("%d", duration.Milliseconds()),
				observe.DataMessages: fmt.Sprintf("%d", len(t.GetHistories())),
			},
			Time: time.Now(),
		})
	}
}

// Notify sends notifications to websocket for various events.
func (t *genericRequestor) Notify(ctx context.Context, actionDatas ...internal_type.Stream) error {
	for _, actionData := range actionDatas {
		t.streamer.Send(actionData)
	}
	return nil
}
