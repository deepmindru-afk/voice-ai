// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package adapter_internal

import (
	"context"

	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
)

// =============================================================================
// OnPacket — enqueue into the priority channel
// =============================================================================

func (r *genericRequestor) OnPacket(ctx context.Context, pkts ...internal_type.Packet) error {
	for _, p := range pkts {
		e := packetEnvelope{ctx: ctx, pkt: p}
		switch p.(type) {
		// Critical — interrupts, tool lifecycle
		case internal_type.InterruptionDetectedPacket,
			internal_type.TTSInterruptPacket,
			internal_type.LLMInterruptPacket,
			internal_type.STTInterruptPacket,
			internal_type.TurnChangePacket:
			r.criticalCh <- e

		// Input — inbound audio pipeline, VAD, STT, EOS
		case internal_type.UserAudioReceivedPacket,
			internal_type.UserTextReceivedPacket,
			internal_type.DenoiseAudioPacket,
			internal_type.DenoisedAudioPacket,
			internal_type.VadAudioPacket,
			internal_type.VadSpeechActivityPacket,
			internal_type.SpeechToTextPacket,
			internal_type.EndOfSpeechPacket,
			internal_type.InterimEndOfSpeechPacket,
			internal_type.UserInputPacket,
			internal_type.LLMToolResultPacket:
			r.inputCh <- e

		// Output — LLM generation, TTS, outbound pipeline
		case internal_type.LLMResponseDeltaPacket,
			internal_type.LLMResponseDonePacket,
			internal_type.ErrorPacket,
			internal_type.InjectMessagePacket,
			internal_type.StartIdleTimeoutPacket,
			internal_type.StopIdleTimeoutPacket,
			internal_type.TTSTextPacket,
			internal_type.TTSDonePacket,
			internal_type.TextToSpeechAudioPacket,
			internal_type.TextToSpeechEndPacket,
			internal_type.LLMToolCallPacket:
			r.outputCh <- e

		// Low — recording, metrics, persistence, events, completion
		case internal_type.RecordUserAudioPacket,
			internal_type.RecordAssistantAudioPacket,
			internal_type.MessageCreatePacket,
			internal_type.ToolLogCreatePacket,
			internal_type.ToolLogUpdatePacket,
			internal_type.WebhookLogCreatePacket,
			internal_type.InitAssistantPacket,
			internal_type.InitConversationPacket,
			internal_type.InitServicePacket,
			internal_type.InitAuthenticatePacket,
			internal_type.InitAudioPacket,
			internal_type.InitBehaviorPacket,
			internal_type.InitializationErrorPacket,
			internal_type.DisconnectCloseIOPacket,
			internal_type.DisconnectRecordingPacket,
			internal_type.DisconnectObservePacket,
			internal_type.DisconnectShutdownPacket,
			internal_type.ExecuteAnalysisPacket,
			internal_type.AnalysisDonePacket,
			internal_type.ExecuteWebhookPacket,
			internal_type.AnalysisStartPacket,
			internal_type.WebhookStartPacket,
			internal_type.WebhookDonePacket,
			internal_type.ConversationMetricPacket,
			internal_type.ConversationMetadataPacket,
			internal_type.AssistantMessageMetricPacket,
			internal_type.UserMessageMetricPacket,
			internal_type.UserMessageMetadataPacket,
			internal_type.AssistantMessageMetadataPacket,
			internal_type.ConversationEventPacket:
			r.lowCh <- e

		default:
			r.logger.Warnf("OnPacket: unrouted packet type %T, falling back to inputCh", p)
			r.inputCh <- e
		}
	}
	return nil
}

// =============================================================================
// Dispatchers — one goroutine per priority channel
// =============================================================================

func (r *genericRequestor) runCriticalDispatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-r.criticalCh:
			r.dispatch(e.ctx, e.pkt)
		}
	}
}

func (r *genericRequestor) runInputDispatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-r.inputCh:
			r.dispatch(e.ctx, e.pkt)
		}
	}
}

func (r *genericRequestor) runOutputDispatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-r.outputCh:
			r.dispatch(e.ctx, e.pkt)
		}
	}
}

func (r *genericRequestor) runLowDispatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-r.lowCh:
			r.dispatch(e.ctx, e.pkt)
		}
	}
}

// =============================================================================
// dispatch — routes a single packet to its handler
// =============================================================================

func (r *genericRequestor) dispatch(ctx context.Context, p internal_type.Packet) {
	switch vl := p.(type) {
	// Input
	case internal_type.UserTextReceivedPacket:
		r.handleUserText(ctx, vl)
	case internal_type.UserAudioReceivedPacket:
		r.handleUserAudio(ctx, vl)
	case internal_type.DenoiseAudioPacket:
		r.handleDenoise(ctx, vl)
	case internal_type.DenoisedAudioPacket:
		r.handleDenoisedAudio(ctx, vl)
	case internal_type.VadAudioPacket:
		r.handleVadAudio(ctx, vl)
	case internal_type.VadSpeechActivityPacket:
		r.callEndOfSpeech(ctx, vl)
	case internal_type.SpeechToTextPacket:
		r.handleSpeechToText(ctx, vl)
	case internal_type.InterimEndOfSpeechPacket:
		r.handleInterimEndOfSpeech(ctx, vl)
	case internal_type.EndOfSpeechPacket:
		r.handleEndOfSpeech(ctx, vl)
	case internal_type.UserInputPacket:
		r.handleUserInput(ctx, vl)

	// Critical
	case internal_type.InterruptionDetectedPacket:
		r.handleInterruption(ctx, vl)
	case internal_type.TTSInterruptPacket:
		r.handleInterruptTTS(ctx, vl)
	case internal_type.LLMInterruptPacket:
		r.handleInterruptLLM(ctx, vl)
	case internal_type.STTInterruptPacket:
		r.handleInterruptSTT(ctx, vl)
	case internal_type.TurnChangePacket:
		r.handleContextChange(ctx, vl)

	// Output
	case internal_type.LLMResponseDeltaPacket:
		r.handleLLMDelta(ctx, vl)
	case internal_type.LLMResponseDonePacket:
		r.handleLLMDone(ctx, vl)
	case internal_type.ErrorPacket:
		r.handleErrorPacket(ctx, vl)
	case internal_type.InjectMessagePacket:
		r.handleInjectMessagePacket(ctx, vl)
	case internal_type.StartIdleTimeoutPacket:
		r.handleStartIdleTimeoutPacket(ctx, vl)
	case internal_type.StopIdleTimeoutPacket:
		r.handleStopIdleTimeoutPacket(ctx, vl)
	case internal_type.TTSTextPacket:
		r.handleTTSText(ctx, vl)
	case internal_type.TTSDonePacket:
		r.handleTTSDone(ctx, vl)
	case internal_type.TextToSpeechAudioPacket:
		r.handleTTSAudio(ctx, vl)
	case internal_type.TextToSpeechEndPacket:
		r.handleTTSEnd(ctx, vl)
	case internal_type.LLMToolCallPacket:
		r.handleToolCall(ctx, vl)
	case internal_type.LLMToolResultPacket:
		r.handleToolResult(ctx, vl)

	// Low
	case internal_type.RecordUserAudioPacket:
		r.handleRecordUserAudio(ctx, vl)
	case internal_type.RecordAssistantAudioPacket:
		r.handleRecordAssistantAudio(ctx, vl)
	case internal_type.MessageCreatePacket:
		r.handleSaveMessage(ctx, vl)
	case internal_type.ConversationMetricPacket:
		r.handleConversationMetric(ctx, vl)
	case internal_type.ConversationMetadataPacket:
		r.handleConversationMetadata(ctx, vl)
	case internal_type.UserMessageMetricPacket:
		r.handleUserMessageMetric(ctx, vl)
	case internal_type.AssistantMessageMetricPacket:
		r.handleAssistantMessageMetric(ctx, vl)
	case internal_type.UserMessageMetadataPacket:
		r.handleUserMessageMetadata(ctx, vl)
	case internal_type.AssistantMessageMetadataPacket:
		r.handleAssistantMessageMetadata(ctx, vl)
	case internal_type.ToolLogCreatePacket:
		r.handleToolLogCreate(ctx, vl)
	case internal_type.ToolLogUpdatePacket:
		r.handleToolLogUpdate(ctx, vl)
	case internal_type.WebhookLogCreatePacket:
		r.handleWebhookLogCreate(ctx, vl)
	case internal_type.ConversationEventPacket:
		r.handleConversationEvent(ctx, vl)

	// Init chain
	case internal_type.InitAssistantPacket:
		r.handleInitAssistant(ctx, vl)
	case internal_type.InitConversationPacket:
		r.handleInitConversation(ctx, vl)
	case internal_type.InitServicePacket:
		r.handleInitService(ctx, vl)
	case internal_type.InitAuthenticatePacket:
		r.handleInitAuthenticate(ctx, vl)
	case internal_type.InitAudioPacket:
		r.handleInitAudio(ctx, vl)
	case internal_type.InitBehaviorPacket:
		r.handleInitBehavior(ctx, vl)

	// Disconnect chain
	case internal_type.DisconnectCloseIOPacket:
		r.handleDisconnectCloseIO(ctx, vl)
	case internal_type.DisconnectRecordingPacket:
		r.handleDisconnectRecording(ctx, vl)
	case internal_type.DisconnectObservePacket:
		r.handleDisconnectObserve(ctx, vl)
	case internal_type.DisconnectShutdownPacket:
		r.handleDisconnectShutdown(ctx, vl)

	// Analysis + Webhook chain
	case internal_type.AnalysisStartPacket:
		r.handleAnalysisStartPacket(ctx, vl)
	case internal_type.ExecuteAnalysisPacket:
		r.handleExecuteAnalysisPacket(ctx, vl)
	case internal_type.AnalysisDonePacket:
		r.handleAnalysisDonePacket(ctx, vl)
	case internal_type.WebhookStartPacket:
		r.handleWebhookStartPacket(ctx, vl)
	case internal_type.ExecuteWebhookPacket:
		r.handleExecuteWebhookPacket(ctx, vl)
	case internal_type.WebhookDonePacket:
		r.handleWebhookDonePacket(ctx, vl)

	default:
		r.logger.Warnf("unknown packet type received in dispatcher %T", vl)
	}
}
