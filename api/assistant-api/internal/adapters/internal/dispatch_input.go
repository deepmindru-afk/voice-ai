// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package adapter_internal

import (
	"context"
	"errors"
	"time"

	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	type_enums "github.com/rapidaai/pkg/types/enums"
	"github.com/rapidaai/pkg/utils"
	"github.com/rapidaai/protos"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (talking *genericRequestor) callEndOfSpeech(ctx context.Context, vl internal_type.Packet) error {
	if talking.endOfSpeech != nil {
		utils.Go(ctx, func() {
			if err := talking.endOfSpeech.Analyze(ctx, vl); err != nil {
				talking.logger.Errorf("end of speech analyze error: %v", err)
			}
		})
		return nil
	}
	return errors.New("end of speech analyzer not configured")
}

func (talking *genericRequestor) handleEndOfSpeech(ctx context.Context, vl internal_type.EndOfSpeechPacket) {
	if err := talking.callInputNormalizer(ctx, vl); err != nil {
		talking.OnPacket(ctx, internal_type.UserInputPacket{
			ContextID: vl.ContextID,
			Text:      vl.Speech,
		})
	}
}

func (talking *genericRequestor) handleInterimEndOfSpeech(ctx context.Context, vl internal_type.InterimEndOfSpeechPacket) {
	talking.Notify(ctx, &protos.ConversationUserMessage{
		Id:        talking.GetID(),
		Message:   &protos.ConversationUserMessage_Text{Text: vl.Speech},
		Completed: false,
		Time:      timestamppb.New(time.Now()),
	})
}

func (talking *genericRequestor) handleUserText(ctx context.Context, vl internal_type.UserTextReceivedPacket) {
	talking.handleInterruption(ctx, internal_type.InterruptionDetectedPacket{
		ContextID: talking.GetID(),
		Source:    internal_type.InterruptionSourceWord,
	})

	vl.ContextID = talking.GetID()
	if err := talking.callEndOfSpeech(ctx, vl); err != nil {
		talking.OnPacket(ctx, internal_type.EndOfSpeechPacket{ContextID: vl.ContextID, Speech: vl.Text})
	}
}

func (talking *genericRequestor) handleUserAudio(ctx context.Context, vl internal_type.UserAudioReceivedPacket) {
	if talking.denoiser != nil && !vl.NoiseReduced {
		talking.OnPacket(ctx, internal_type.DenoiseAudioPacket{ContextID: vl.ContextID, Audio: vl.Audio})
		return
	}
	talking.OnPacket(ctx,
		internal_type.RecordUserAudioPacket{ContextID: vl.ContextID, Audio: vl.Audio},
		internal_type.VadAudioPacket{ContextID: vl.ContextID, Audio: vl.Audio},
	)
	if talking.speechToTextTransformer != nil {
		utils.Go(ctx, func() {
			if err := talking.speechToTextTransformer.Transform(ctx, vl); err != nil {
				talking.logger.Tracef(ctx, "error while transforming input %s and error %s", talking.speechToTextTransformer.Name(), err.Error())
			}
		})
	}
	talking.callEndOfSpeech(ctx, vl)
}

func (talking *genericRequestor) handleDenoise(ctx context.Context, vl internal_type.DenoiseAudioPacket) {
	if talking.denoiser != nil {
		if err := talking.denoiser.Denoise(ctx, vl); err != nil {
			talking.logger.Warnf("denoiser returned unexpected error: %+v", err)
		}
	}
}

func (talking *genericRequestor) handleDenoisedAudio(ctx context.Context, vl internal_type.DenoisedAudioPacket) {
	talking.OnPacket(ctx, internal_type.UserAudioReceivedPacket{
		ContextID:    vl.ContextID,
		Audio:        vl.Audio,
		NoiseReduced: true,
	})
}

func (talking *genericRequestor) handleVadAudio(ctx context.Context, vl internal_type.VadAudioPacket) {
	if talking.vad != nil {
		utils.Go(ctx, func() {
			if err := talking.vad.Process(ctx, internal_type.UserAudioReceivedPacket{ContextID: vl.ContextID, Audio: vl.Audio}); err != nil {
				talking.logger.Warnf("error while processing with vad %s", err.Error())
			}
		})
	}
}

func (talking *genericRequestor) handleSpeechToText(ctx context.Context, vl internal_type.SpeechToTextPacket) {
	vl.ContextID = talking.GetID()
	if err := talking.callEndOfSpeech(ctx, vl); err != nil {
		if !vl.Interim {
			talking.OnPacket(ctx, internal_type.EndOfSpeechPacket{
				ContextID: vl.ContextID,
				Speech:    vl.Script,
				Speechs:   []internal_type.SpeechToTextPacket{vl},
			})
		}
	}
}

func (talking *genericRequestor) callInputNormalizer(ctx context.Context, vl internal_type.EndOfSpeechPacket) error {
	if talking.inputNormalizer == nil {
		return errors.New("input inputNormalizer not configured")
	}
	if err := talking.inputNormalizer.Normalize(ctx, vl); err != nil {
		talking.logger.Errorf("input inputNormalizer error: %v", err)
		return err
	}
	return nil
}

func (talking *genericRequestor) handleUserInput(ctx context.Context, vl internal_type.UserInputPacket) {
	talking.OnPacket(ctx, internal_type.StopIdleTimeoutPacket{
		ContextID: talking.GetID(), ResetCount: true,
	})

	if err := talking.Transition(LLMGenerating); err != nil {
		talking.logger.Errorf("messaging transition error: %v", err)
	}

	contextID := talking.GetID()
	vl.ContextID = contextID

	if err := talking.Notify(ctx, &protos.ConversationUserMessage{
		Id:        contextID,
		Message:   &protos.ConversationUserMessage_Text{Text: vl.Text},
		Completed: true,
		Time:      timestamppb.New(time.Now()),
	}); err != nil {
		talking.logger.Tracef(ctx, "might be returning processing the duplicate message so cut it out.")
		return
	}
	talking.OnPacket(ctx,
		internal_type.MessageCreatePacket{ContextID: contextID, MessageRole: "user", Text: vl.Text},
		internal_type.UserMessageMetadataPacket{ContextID: contextID, Metadata: []*protos.Metadata{
			{
				Key:   "language",
				Value: vl.Language.Name,
			},
			{
				Key:   "language_code",
				Value: vl.Language.ISO639_1,
			}}},
		internal_type.UserMessageMetricPacket{ContextID: contextID, Metrics: []*protos.Metric{{Name: "user_turn", Value: type_enums.CONVERSATION_COMPLETE.String(), Description: "User turn started"}}})

	if talking.assistantExecutor != nil {
		utils.Go(ctx, func() {
			if err := talking.assistantExecutor.Execute(ctx, talking, vl); err != nil {
				talking.OnPacket(ctx, internal_type.LLMErrorPacket{ContextID: contextID, Error: err})
			}
		})
	}
}
