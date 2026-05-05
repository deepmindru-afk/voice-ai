// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package adapter_internal

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	internal_condition "github.com/rapidaai/api/assistant-api/internal/condition"
	observe "github.com/rapidaai/api/assistant-api/internal/observe"
	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	"github.com/rapidaai/api/assistant-api/internal/variable"
	internal_namespace "github.com/rapidaai/api/assistant-api/internal/variable/namespace"
	"github.com/rapidaai/pkg/utils"
)

// =============================================================================
// Disconnect chain handlers
// =============================================================================

// handleDisconnectCloseIO shuts down all I/O subsystems concurrently and enqueues
// the next step in the disconnect chain. This handler blocks the low dispatcher
// goroutine via wg.Wait(), which is intentional: at disconnect time no new low
// priority packets need processing, and the blocking wait provides a clean
// synchronization point before proceeding to recording persistence.
func (r *genericRequestor) handleDisconnectCloseIO(ctx context.Context, pkt internal_type.DisconnectCloseIOPacket) {
	var wg sync.WaitGroup
	wg.Add(4)
	utils.Go(ctx, func() {
		defer wg.Done()
		if err := r.disconnectSpeechToText(ctx); err != nil {
			r.logger.Tracef(ctx, "failed to close input transformer: %+v", err)
		}
		if err := r.disconnectEndOfSpeech(ctx); err != nil {
			r.logger.Tracef(ctx, "failed to close end of speech: %+v", err)
		}
	})
	utils.Go(ctx, func() {
		defer wg.Done()
		if err := r.disconnectTextToSpeech(ctx); err != nil {
			r.logger.Tracef(ctx, "failed to close output transformer: %+v", err)
		}
	})
	utils.Go(ctx, func() {
		defer wg.Done()
		r.disconnectOutputNormalizer(ctx)
	})
	utils.Go(ctx, func() {
		defer wg.Done()
		r.disconnectInputNormalizer(ctx)
	})
	wg.Wait()
	r.OnPacket(ctx, internal_type.DisconnectRecordingPacket{ContextID: pkt.ContextID})
}

func (r *genericRequestor) handleDisconnectRecording(ctx context.Context, pkt internal_type.DisconnectRecordingPacket) {
	if r.recorder != nil {
		utils.Go(ctx, func() {
			userAudio, systemAudio, err := r.recorder.Persist()
			if err != nil {
				r.logger.Tracef(ctx, "failed to persist audio recording: %+v", err)
				return
			}
			if err = r.CreateConversationRecording(ctx, userAudio, systemAudio); err != nil {
				r.logger.Tracef(ctx, "failed to create conversation recording record: %+v", err)
			}
		})
	}

	// Enqueue analysis+webhooks with a Done channel. A goroutine waits for the
	// channel to close (signaling that analysis and webhooks have finished) and
	// then explicitly enqueues the next disconnect chain step. This keeps the
	// continuation logic here in the disconnect chain rather than implicitly
	// buried inside handleWebhookDonePacket.
	done := make(chan struct{}, 1)
	r.OnPacket(ctx, internal_type.AnalysisStartPacket{ContextID: pkt.ContextID, Done: done})
	utils.Go(ctx, func() {
		<-done
		r.OnPacket(ctx, internal_type.DisconnectObservePacket{ContextID: pkt.ContextID})
	})
}

func (r *genericRequestor) handleDisconnectObserve(ctx context.Context, pkt internal_type.DisconnectObservePacket) {
	if r.observer != nil {
		r.observer.EventCollectors().Collect(ctx, observe.EventRecord{
			ConversationID: r.observer.Meta().AssistantConversationID,
			MessageID:      r.GetID(),
			Name:           observe.ComponentSession,
			Data:           map[string]string{observe.DataType: observe.EventDisconnected, observe.DataMessages: fmt.Sprintf("%d", len(r.GetHistories()))},
			Time:           time.Now(),
		})
	}
	r.shutdownCollectors(ctx)
	r.OnPacket(ctx, internal_type.DisconnectShutdownPacket{ContextID: pkt.ContextID})
}

func (r *genericRequestor) handleDisconnectShutdown(ctx context.Context, _ internal_type.DisconnectShutdownPacket) {
	if err := r.assistantExecutor.Close(ctx); err != nil {
		r.logger.Errorf("failed to close assistant executor: %v", err)
	}
	if r.idleTimeoutTimer != nil {
		r.idleTimeoutTimer.Stop()
	}
	if r.maxSessionTimer != nil {
		r.maxSessionTimer.Stop()
	}
	if r.disconnectDone != nil {
		close(r.disconnectDone)
	}
}

// =============================================================================
// Analysis + Webhook chain handlers
// =============================================================================

func (r *genericRequestor) handleAnalysisStartPacket(ctx context.Context, pkt internal_type.AnalysisStartPacket) {
	source := variable.NewCommunicationSource(r)
	registry := internal_namespace.NewDefaultRegistry().With("event", &internal_namespace.EventNamespace{})
	direction := r.Conversation().Direction.String()
	for _, analysis := range r.assistant.AssistantAnalyses {
		if !r.isConditionAllowed(analysis.GetOptions(), "analysis.condition", direction) {
			continue
		}
		r.OnPacket(ctx, internal_type.ExecuteAnalysisPacket{
			ContextID:      pkt.ContextID,
			Analysis:       analysis,
			Arguments:      registry.Apply(analysis.GetParameters(), source, variable.ResolveContext{Event: utils.ConversationCompleted.Get()}),
			ConversationID: r.assistantConversation.Id,
			Auth:           r.auth,
		})
	}
	r.OnPacket(ctx, internal_type.AnalysisDonePacket{
		ContextID: pkt.ContextID,
		Event:     utils.ConversationCompleted,
		Done:      pkt.Done,
	})
}

func (r *genericRequestor) handleAnalysisDonePacket(ctx context.Context, pkt internal_type.AnalysisDonePacket) {
	r.OnPacket(ctx, internal_type.WebhookStartPacket{
		ContextID: pkt.ContextID,
		Event:     pkt.Event,
		Done:      pkt.Done,
	})
}

func (r *genericRequestor) handleExecuteAnalysisPacket(ctx context.Context, packet internal_type.ExecuteAnalysisPacket) {
	if r.analysisExecutor == nil {
		return
	}
	if err := r.analysisExecutor.Execute(ctx, packet); err != nil {
		r.logger.Warnw("analysis execution failed", "name", packet.Analysis.GetName(), "error", err)
	}
}

func (r *genericRequestor) handleWebhookStartPacket(ctx context.Context, pkt internal_type.WebhookStartPacket) {
	source := variable.NewCommunicationSource(r)
	registry := internal_namespace.NewDefaultRegistry().With("event", &internal_namespace.EventNamespace{})
	direction := r.Conversation().Direction.String()
	for _, webhook := range r.assistant.AssistantWebhooks {
		if !slices.Contains(webhook.GetAssistantEvents(), pkt.Event.Get()) {
			continue
		}
		if !r.isConditionAllowed(webhook.GetOptions(), "webhook.condition", direction) {
			continue
		}
		r.OnPacket(ctx, internal_type.ExecuteWebhookPacket{
			ContextID: pkt.ContextID,
			Event:     pkt.Event,
			Webhook:   webhook,
			Arguments: registry.Apply(webhook.GetBody(), source, variable.ResolveContext{Event: pkt.Event.Get()}),
		})
	}
	r.OnPacket(ctx, internal_type.WebhookDonePacket{
		ContextID: pkt.ContextID,
		Done:      pkt.Done,
	})
}

func (r *genericRequestor) handleExecuteWebhookPacket(ctx context.Context, rwp internal_type.ExecuteWebhookPacket) {
	if r.webhookExecutor == nil {
		return
	}
	if err := r.webhookExecutor.Execute(ctx, rwp); err != nil {
		r.logger.Warnw("webhook execution failed", "webhookID", rwp.Webhook.Id, "error", err)
	}
}

// handleWebhookDonePacket closes the Done channel if provided, signaling any
// goroutine waiting on it that webhooks have completed. When Done is nil (fire-
// and-forget webhooks from begin/resume/failed events), this is a terminal no-op.
// The disconnect chain continuation is handled by the goroutine spawned in
// handleDisconnectRecording, NOT here — keeping packet routing explicit.
func (r *genericRequestor) handleWebhookDonePacket(_ context.Context, pkt internal_type.WebhookDonePacket) {
	if pkt.Done != nil {
		close(pkt.Done)
	}
}

// =============================================================================
// Error — fires ConversationFailed webhooks
// =============================================================================

// RunError fires ConversationFailed webhooks with a nil Done channel, making
// them fire-and-forget. No further disconnect chain steps are triggered — the
// session cleanup happens when the stream closes and Disconnect is called normally.
func (r *genericRequestor) RunError(ctx context.Context, contextID string) {
	r.OnPacket(ctx, internal_type.WebhookStartPacket{
		ContextID: contextID,
		Event:     utils.ConversationFailed,
	})
}

// =============================================================================
// Condition filter
// =============================================================================

func (r *genericRequestor) isConditionAllowed(opts utils.Option, key string, direction string) bool {
	raw, err := opts.GetString(key)
	if err != nil {
		return true
	}
	parsed, parseErr := internal_condition.Parse(raw)
	if parseErr != nil {
		r.logger.Warnf("invalid %s: %v", key, parseErr)
		return false
	}
	allowed, evalErr := parsed.Run(
		internal_condition.ConditionValue{RuleType: internal_condition.RuleTypeSource, Value: r.GetSource().Get()},
		internal_condition.ConditionValue{RuleType: internal_condition.RuleTypeMode, Value: r.GetMode().String()},
		internal_condition.ConditionValue{RuleType: internal_condition.RuleTypeDirection, Value: direction},
	)
	if evalErr != nil {
		r.logger.Warnf("condition eval failed for %s: %v", key, evalErr)
		return false
	}
	return allowed
}
