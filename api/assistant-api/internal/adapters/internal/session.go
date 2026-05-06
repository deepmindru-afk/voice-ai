// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.

// Package internal_adapter_generic provides the generic adapter implementation
// for managing voice assistant sessions. It handles the complete lifecycle of
// assistant conversations including connection, disconnection, audio streaming,
// and state management.
package adapter_internal

import (
	"context"
	"time"

	observe "github.com/rapidaai/api/assistant-api/internal/observe"
	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	"github.com/rapidaai/pkg/types"
	"github.com/rapidaai/protos"
)

// =============================================================================
// Constants
// =============================================================================

const (
	dbWriteTimeout        = 5 * time.Second
	collectorWriteTimeout = 10 * time.Second
)

// =============================================================================
// Session Lifecycle
// =============================================================================

// Connect starts bootstrap/background dispatchers and enqueues the init chain.
// Runtime dispatchers (critical/ingress/egress) are started after
// InitializationCompleted. Connect always returns nil because initialization
// runs asynchronously on the bootstrap dispatcher goroutine.
// The gRPC stream is already open by the time Connect is called; any init errors
// are delivered to the client via InitializationFailedPacket → ConversationError
// proto on the stream, not via this return value.
func (r *genericRequestor) Connect(ctx context.Context, auth types.SimplePrinciple, config *protos.ConversationInitialization) error {
	r.SetAuth(auth)
	go r.runBootstrapDispatcher(ctx)
	go r.runLowDispatcher(r.workerCtx)
	r.OnPacket(ctx,
		internal_type.ConversationEventPacket{
			ContextID: r.GetID(),
			Name:      observe.ComponentSession,
			Data:      map[string]string{observe.DataType: observe.EventInitializing, observe.DataMode: config.GetStreamMode().String()},
			Time:      time.Now(),
		}, internal_type.InitializeAssistantPacket{
			ContextID: r.GetID(),
			Config:    config,
		})
	return nil
}

// Disconnect enqueues the disconnect chain and blocks until complete.
// The disconnectDone channel is created fresh here and closed exactly once by
// handleFinalizationCompleted — the terminal step of the disconnect chain.
// Disconnect is called at most once per session (guarded by the gRPC stream
// lifecycle), so there is no risk of double-close.
func (r *genericRequestor) Disconnect(ctx context.Context) {
	startTime := time.Now()
	done := make(chan struct{}, 1)
	r.disconnectDone = done
	r.OnPacket(ctx, internal_type.FinalizeBehaviorPacket{ContextID: r.GetID()})
	select {
	case <-done:
	case <-time.After(collectorWriteTimeout):
		r.logger.Warnf("disconnect timed out after %v", collectorWriteTimeout)
	}
	r.workerCancel()
	r.logger.Benchmark("session.Disconnect", time.Since(startTime))
}
