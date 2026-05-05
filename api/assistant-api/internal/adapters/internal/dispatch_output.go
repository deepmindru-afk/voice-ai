// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package adapter_internal

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	internal_audio "github.com/rapidaai/api/assistant-api/internal/audio"
	observe "github.com/rapidaai/api/assistant-api/internal/observe"
	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	type_enums "github.com/rapidaai/pkg/types/enums"
	"github.com/rapidaai/pkg/utils"
	"github.com/rapidaai/protos"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (talking *genericRequestor) handleLLMDelta(ctx context.Context, vl internal_type.LLMResponseDeltaPacket) {
	if vl.ContextID != talking.GetID() {
		talking.OnPacket(ctx, internal_type.ConversationEventPacket{
			ContextID: vl.ContextID,
			Name:      "llm",
			Data:      map[string]string{"type": "discarded", "reason": "stale_context", "current_context": talking.GetID(), "text": vl.Text},
			Time:      time.Now(),
		})
		return
	}
	if err := talking.Transition(LLMGenerating); err != nil {
		talking.logger.Errorf("messaging transition error: %v", err)
	}
	if talking.outputNormalizer != nil {
		talking.outputNormalizer.Normalize(ctx, vl)
	} else {
		talking.OnPacket(ctx, internal_type.TTSTextPacket{ContextID: vl.ContextID, Text: vl.Text})
	}
}

func (talking *genericRequestor) handleLLMDone(ctx context.Context, vl internal_type.LLMResponseDonePacket) {
	if vl.ContextID != talking.GetID() {
		talking.OnPacket(ctx, internal_type.ConversationEventPacket{
			ContextID: vl.ContextID,
			Name:      "llm",
			Data:      map[string]string{"type": "discarded", "reason": "stale_context", "packet": "done", "current_context": talking.GetID(), "text": vl.Text},
			Time:      time.Now(),
		})
		return
	}
	talking.OnPacket(ctx, internal_type.StartIdleTimeoutPacket{ContextID: vl.ContextID})
	if err := talking.Transition(LLMGenerated); err != nil {
		talking.logger.Errorf("messaging transition error: %v", err)
	}
	talking.OnPacket(ctx,
		internal_type.MessageCreatePacket{ContextID: vl.ContextID, MessageRole: "assistant", Text: vl.Text},
		internal_type.AssistantMessageMetricPacket{
			ContextID: vl.ContextID,
			Metrics:   []*protos.Metric{{Name: "assistant_turn", Value: type_enums.CONVERSATION_COMPLETE.String(), Description: fmt.Sprintf("LLM response completed")}},
		},
	)
	if talking.outputNormalizer != nil {
		talking.outputNormalizer.Normalize(ctx, vl)
	} else {
		talking.OnPacket(ctx, internal_type.TTSDonePacket{ContextID: vl.ContextID, Text: vl.Text})
	}
}

func (talking *genericRequestor) handleErrorPacket(ctx context.Context, vl internal_type.ErrorPacket) {
	switch vl.(type) {
	case internal_type.InitializationErrorPacket:
		talking.OnPacket(ctx, internal_type.ConversationEventPacket{
			ContextID: vl.ContextId(),
			Name:      "session",
			Data:      map[string]string{"type": "error", "message": vl.ErrMessage()},
			Time:      time.Now(),
		})
	case internal_type.LLMErrorPacket:
		talking.OnPacket(ctx,
			internal_type.UserMessageMetricPacket{
				ContextID: vl.ContextId(),
				Metrics: []*protos.Metric{{
					Name:        "llm_error",
					Value:       vl.ErrMessage(),
					Description: "An error occurred during LLM processing"}},
			},
			internal_type.ConversationEventPacket{
				ContextID: vl.ContextId(),
				Name:      "llm",
				Data:      map[string]string{"type": "error", "message": vl.ErrMessage()},
				Time:      time.Now(),
			})
		talking.Transition(LLMGenerated)
	case internal_type.STTErrorPacket:
		talking.OnPacket(ctx,
			internal_type.UserMessageMetricPacket{
				ContextID: vl.ContextId(),
				Metrics: []*protos.Metric{{
					Name:        "stt_error",
					Value:       vl.ErrMessage(),
					Description: "An error occurred during STT processing"}},
			},
			internal_type.ConversationEventPacket{
				ContextID: vl.ContextId(),
				Name:      "stt",
				Data:      map[string]string{"type": "error", "message": vl.ErrMessage()},
				Time:      time.Now(),
			})
	case internal_type.TTSErrorPacket:
		talking.OnPacket(ctx,
			internal_type.UserMessageMetricPacket{
				ContextID: vl.ContextId(),
				Metrics: []*protos.Metric{{
					Name:        "tts_error",
					Value:       vl.ErrMessage(),
					Description: "An error occurred during TTS processing"}},
			},
			internal_type.ConversationEventPacket{
				ContextID: vl.ContextId(),
				Name:      "tts",
				Data:      map[string]string{"type": "error", "message": vl.ErrMessage()},
				Time:      time.Now(),
			})
	}
	if !vl.IsRecoverable() {
		talking.RunError(ctx, vl.ContextId())
		var conversationId uint64
		if talking.Conversation() != nil {
			conversationId = talking.Conversation().Id
		}
		talking.Notify(ctx,
			&protos.ConversationError{
				AssistantConversationId: conversationId,
				Message:                 vl.ErrMessage(),
			},
			&protos.ConversationDisconnection{
				Type: protos.ConversationDisconnection_DISCONNECTION_TYPE_UNSPECIFIED,
			})
		return
	}
	_ = talking.Notify(ctx, &protos.ConversationError{
		AssistantConversationId: talking.Conversation().Id,
		Message:                 vl.ErrMessage(),
	})
}

func (talking *genericRequestor) handleInjectMessagePacket(ctx context.Context, vl internal_type.InjectMessagePacket) {
	if err := talking.Transition(LLMGenerating); err != nil {
		talking.logger.Errorf("messaging transition error: %v", err)
	}

	if talking.assistantExecutor != nil {
		utils.Go(ctx, func() {
			if err := talking.assistantExecutor.Execute(ctx, talking, vl); err != nil {
				talking.logger.Errorf("assistant executor error: %v", err)
			}
		})
	}

	contextID := talking.GetID()

	if talking.outputNormalizer != nil {
		talking.OnPacket(ctx,
			internal_type.MessageCreatePacket{ContextID: contextID, MessageRole: "assistant", Text: vl.Text},
			internal_type.AssistantMessageMetricPacket{
				ContextID: contextID,
				Metrics:   []*protos.Metric{{Name: "assistant_turn", Value: type_enums.CONVERSATION_COMPLETE.String(), Description: "Injected message completed"}},
			},
		)
		talking.outputNormalizer.Normalize(ctx, internal_type.InjectMessagePacket{ContextID: contextID, Text: vl.Text})
		if err := talking.Transition(LLMGenerated); err != nil {
			talking.logger.Errorf("messaging transition error: %v", err)
		}
	} else {
		talking.OnPacket(ctx,
			internal_type.LLMResponseDeltaPacket{ContextID: contextID, Text: vl.Text},
			internal_type.LLMResponseDonePacket{ContextID: contextID, Text: vl.Text},
		)
	}
}

func (talking *genericRequestor) handleStartIdleTimeoutPacket(ctx context.Context, vl internal_type.StartIdleTimeoutPacket) {
	if talking.idleTimeoutTimer != nil {
		talking.idleTimeoutTimer.Stop()
	}
	behavior, err := talking.GetBehavior()
	if err != nil {
		return
	}
	if behavior.IdleTimeout == nil || *behavior.IdleTimeout == 0 {
		return
	}

	timeoutDuration := time.Duration(*behavior.IdleTimeout) * time.Second
	talking.idleTimeoutDeadline = time.Now().Add(timeoutDuration)
	talking.idleTimeoutTimer = time.AfterFunc(timeoutDuration, func() {
		if err := talking.onIdleTimeout(ctx); err != nil {
			talking.logger.Errorf("error while handling idle timeout: %v", err)
		}
	})
}

func (talking *genericRequestor) handleStopIdleTimeoutPacket(ctx context.Context, vl internal_type.StopIdleTimeoutPacket) {
	if talking.idleTimeoutTimer != nil {
		talking.idleTimeoutTimer.Stop()
		talking.idleTimeoutTimer = nil
	}
	talking.idleTimeoutDeadline = time.Time{}

	if vl.ResetCount {
		talking.idleTimeoutCount = 0
	}
}

func (talking *genericRequestor) handleTTSText(ctx context.Context, vl internal_type.TTSTextPacket) {
	if vl.ContextID != talking.GetID() {
		return
	}
	if talking.textToSpeechTransformer != nil && talking.GetMode().Audio() {
		if err := talking.textToSpeechTransformer.Transform(ctx, vl); err != nil {
			talking.logger.Errorf("tts text: failed to send chunk: %v", err)
		}
	}
	talking.Notify(ctx, &protos.ConversationAssistantMessage{
		Time: timestamppb.Now(), Id: vl.ContextID, Completed: false,
		Message: &protos.ConversationAssistantMessage_Text{Text: vl.Text},
	})
}

func (talking *genericRequestor) handleTTSDone(ctx context.Context, vl internal_type.TTSDonePacket) {
	if vl.ContextID != talking.GetID() {
		return
	}

	if talking.textToSpeechTransformer != nil && talking.GetMode().Audio() {
		if err := talking.textToSpeechTransformer.Transform(ctx, vl); err != nil {
			talking.logger.Errorf("tts done: failed to send final: %v", err)
		}
	}
	talking.Notify(ctx, &protos.ConversationAssistantMessage{
		Time: timestamppb.Now(), Id: vl.ContextID, Completed: true,
		Message: &protos.ConversationAssistantMessage_Text{Text: vl.Text},
	})
}

func (talking *genericRequestor) handleTTSAudio(ctx context.Context, vl internal_type.TextToSpeechAudioPacket) {
	if talking.GetMode().Audio() {
		audioInfo := internal_audio.GetAudioInfo(vl.AudioChunk, internal_audio.RAPIDA_INTERNAL_AUDIO_CONFIG)
		talking.extendIdleTimeoutTimer(time.Duration(audioInfo.DurationMs) * time.Millisecond)
	}
	if vl.ContextID != talking.GetID() {
		talking.OnPacket(ctx,
			internal_type.ConversationEventPacket{
				ContextID: vl.ContextID,
				Name:      "tts",
				Data:      map[string]string{"type": "discarded", "reason": "stale_context", "packet": "tts_audio", "current_context": talking.GetID()},
				Time:      time.Now(),
			},
			internal_type.AssistantMessageMetricPacket{
				ContextID: vl.ContextID,
				Metrics:   []*protos.Metric{{Name: "discarded_tts_chunk", Value: "true", Description: fmt.Sprintf("tts end packet discarded due to stale contextID %s", talking.GetID())}},
			})
		return
	}
	if err := talking.Notify(ctx, &protos.ConversationAssistantMessage{
		Time:      timestamppb.Now(),
		Id:        vl.ContextID,
		Message:   &protos.ConversationAssistantMessage_Audio{Audio: vl.AudioChunk},
		Completed: false,
	}); err != nil {
		talking.logger.Tracef(ctx, "error while outputting chunk to the user: %w", err)
	}
	talking.OnPacket(ctx, internal_type.RecordAssistantAudioPacket{ContextID: vl.ContextID, Audio: vl.AudioChunk})
}

func (talking *genericRequestor) handleTTSEnd(ctx context.Context, vl internal_type.TextToSpeechEndPacket) {
	if vl.ContextID != talking.GetID() {
		talking.OnPacket(ctx,
			internal_type.ConversationEventPacket{
				ContextID: vl.ContextID,
				Name:      "tts",
				Data:      map[string]string{"type": "discarded", "reason": "stale_context", "packet": "tts_end", "current_context": talking.GetID()},
				Time:      time.Now(),
			},
			internal_type.AssistantMessageMetricPacket{
				ContextID: vl.ContextID,
				Metrics:   []*protos.Metric{{Name: "discarded_tts", Value: "true", Description: fmt.Sprintf("tts end packet discarded due to stale contextID %s", talking.GetID())}},
			})
		return
	}
	if err := talking.Notify(ctx, &protos.ConversationAssistantMessage{
		Time:      timestamppb.Now(),
		Id:        vl.ContextID,
		Completed: true,
	}); err != nil {
		talking.logger.Tracef(ctx, "error while outputting chunk to the user: %w", err)
	}
}

func (talking *genericRequestor) handleToolCall(ctx context.Context, vl internal_type.LLMToolCallPacket) {
	req, _ := json.Marshal(vl)
	talking.OnPacket(ctx, internal_type.ConversationEventPacket{
		ContextID: vl.ContextID,
		Name:      observe.ComponentTool,
		Data:      map[string]string{observe.DataType: observe.EventToolCallStarted, "name": vl.Name, "id": vl.ToolID, "action": vl.Action.String()},
		Time:      time.Now(),
	}, internal_type.ToolLogCreatePacket{
		ContextID: vl.ContextID, ToolID: vl.ToolID, Name: vl.Name, Request: req,
	},
	)

	if msg, ok := vl.Arguments["message"]; ok && msg != "" {
		talking.OnPacket(ctx,
			internal_type.TTSInterruptPacket{ContextID: vl.ContextID},
			internal_type.InjectMessagePacket{ContextID: vl.ContextID, Text: msg})
	}

	if delayStr, ok := vl.Arguments["delay"]; ok && delayStr != "" {
		if delayMs, err := strconv.Atoi(delayStr); err == nil && delayMs > 0 {
			time.AfterFunc(time.Duration(delayMs)*time.Millisecond, func() {
				talking.Notify(ctx, &protos.ConversationToolCall{
					Id: vl.ContextID, ToolId: vl.ToolID, Name: vl.Name,
					Action: vl.Action, Args: vl.Arguments, Time: timestamppb.Now(),
				})
			})
		}
	} else {
		talking.Notify(ctx, &protos.ConversationToolCall{
			Id: vl.ContextID, ToolId: vl.ToolID, Name: vl.Name,
			Action: vl.Action, Args: vl.Arguments, Time: timestamppb.Now(),
		})
	}

	if vl.Action != protos.ToolCallAction_TOOL_CALL_ACTION_UNSPECIFIED {
		talking.OnPacket(ctx, internal_type.StopIdleTimeoutPacket{
			ContextID: talking.GetID(), ResetCount: true,
		})
		if talking.maxSessionTimer != nil {
			talking.maxSessionTimer.Stop()
		}
	}

	if talking.assistantExecutor != nil {
		utils.Go(ctx, func() {
			if err := talking.assistantExecutor.Execute(ctx, talking, vl); err != nil {
				talking.logger.Errorf("assistant executor error: %v", err)
			}
		})
	}
}

func (talking *genericRequestor) handleToolResult(ctx context.Context, vl internal_type.LLMToolResultPacket) {
	res, _ := json.Marshal(vl)

	talking.OnPacket(ctx,
		internal_type.ToolLogUpdatePacket{
			ContextID: vl.ContextID, ToolID: vl.ToolID, Response: res,
		})

	switch vl.Action {
	case protos.ToolCallAction_TOOL_CALL_ACTION_END_CONVERSATION:
		talking.Notify(ctx, &protos.ConversationDisconnection{
			Type: protos.ConversationDisconnection_DISCONNECTION_TYPE_TOOL,
		})
		return
	case protos.ToolCallAction_TOOL_CALL_ACTION_TRANSFER_CONVERSATION:
		if vl.Result["next_action"] == "end_call" {
			talking.Notify(ctx, &protos.ConversationDisconnection{
				Type: protos.ConversationDisconnection_DISCONNECTION_TYPE_TOOL,
			})
			return
		}
	}

	talking.OnPacket(
		ctx,
		internal_type.TTSInterruptPacket{ContextID: vl.ContextID},
		internal_type.StartIdleTimeoutPacket{ContextID: vl.ContextID},
		internal_type.ConversationEventPacket{
			ContextID: vl.ContextID,
			Name:      observe.ComponentTool,
			Data:      map[string]string{observe.DataType: observe.EventToolCallCompleted, "name": vl.Name, "id": vl.ToolID},
			Time:      time.Now(),
		},
	)
	if talking.assistantExecutor != nil {
		utils.Go(ctx, func() {
			if err := talking.assistantExecutor.Execute(ctx, talking, vl); err != nil {
				talking.logger.Errorf("tool result processing failed: %v", err)
			}
		})
	}
}
