// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package adapter_internal

import (
	"context"
	"time"

	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	"github.com/rapidaai/pkg/utils"
	"github.com/rapidaai/protos"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (talking *genericRequestor) handleInterruption(ctx context.Context, vl internal_type.InterruptionDetectedPacket) {
	if vl.ContextID == "" {
		vl.ContextID = talking.GetID()
	}

	switch vl.Source {
	case internal_type.InterruptionSourceWord:
		talking.OnPacket(ctx, internal_type.StopIdleTimeoutPacket{ContextID: vl.ContextID})
		if err := talking.callEndOfSpeech(ctx, vl); err != nil {
			talking.logger.Errorf("end of speech error: %v", err)
		}
		if err := talking.Transition(Interrupted); err != nil {
			return
		}
		talking.OnPacket(ctx,
			internal_type.RecordAssistantAudioPacket{ContextID: vl.ContextID, Truncate: true},
			internal_type.TTSInterruptPacket{ContextID: vl.ContextID, StartAt: vl.StartAt, EndAt: vl.EndAt},
			internal_type.LLMInterruptPacket{ContextID: vl.ContextID},
		)
		utils.Go(ctx, func() {
			talking.Notify(ctx, &protos.ConversationInterruption{
				Type: protos.ConversationInterruption_INTERRUPTION_TYPE_WORD,
				Time: timestamppb.Now(),
			})
		})

	default:
		if vl.StartAt < 5 {
			return
		}

		talking.OnPacket(ctx, internal_type.STTInterruptPacket{ContextID: vl.ContextID})

		if err := talking.callEndOfSpeech(ctx, vl); err != nil {
			talking.logger.Errorf("end of speech error: %v", err)
		}

		if err := talking.Transition(Interrupt); err != nil {
			return
		}
		utils.Go(ctx, func() {
			talking.Notify(ctx, &protos.ConversationInterruption{
				Type: protos.ConversationInterruption_INTERRUPTION_TYPE_VAD,
				Time: timestamppb.Now(),
			})
		})
	}
}

func (talking *genericRequestor) handleContextChange(ctx context.Context, vl internal_type.TurnChangePacket) {
	if vl.ContextID == "" {
		vl.ContextID = talking.GetID()
	}
	if vl.Time.IsZero() {
		vl.Time = time.Now()
	}

	if talking.speechToTextTransformer != nil {
		if err := talking.speechToTextTransformer.Transform(ctx, vl); err != nil {
			talking.logger.Errorf("stt context-change update failed: %v", err)
		}
	}
	if talking.textToSpeechTransformer != nil {
		if err := talking.textToSpeechTransformer.Transform(ctx, vl); err != nil {
			talking.logger.Errorf("tts context-change update failed: %v", err)
		}
	}

	talking.OnPacket(ctx, internal_type.ConversationEventPacket{
		ContextID: vl.ContextID,
		Name:      "turn",
		Data: map[string]string{
			"type":           "change",
			"old_context_id": vl.PreviousContextID,
			"new_context_id": vl.ContextID,
			"reason":         vl.Reason,
			"source":         vl.Source,
		},
		Time: vl.Time,
	})
}

func (talking *genericRequestor) handleInterruptTTS(ctx context.Context, vl internal_type.TTSInterruptPacket) {
	if talking.textToSpeechTransformer != nil {
		if err := talking.textToSpeechTransformer.Transform(ctx, vl); err != nil {
			talking.logger.Errorf("tts interrupt: %v", err)
		}
	}
}

func (talking *genericRequestor) handleInterruptLLM(ctx context.Context, vl internal_type.LLMInterruptPacket) {
	if talking.assistantExecutor != nil {
		if err := talking.assistantExecutor.Execute(ctx, talking, vl); err != nil {
			talking.logger.Errorf("llm interrupt: %v", err)
		}
	}
}

func (talking *genericRequestor) handleInterruptSTT(ctx context.Context, vl internal_type.STTInterruptPacket) {
	if talking.speechToTextTransformer != nil {
		if err := talking.speechToTextTransformer.Transform(ctx, vl); err != nil {
			talking.logger.Errorf("stt interrupt: %v", err)
		}
	}
}
