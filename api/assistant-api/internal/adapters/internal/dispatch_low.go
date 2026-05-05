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
	type_enums "github.com/rapidaai/pkg/types/enums"
	"github.com/rapidaai/protos"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (talking *genericRequestor) handleRecordUserAudio(ctx context.Context, vl internal_type.RecordUserAudioPacket) {
	if talking.recorder != nil {
		if err := talking.recorder.Record(ctx, vl); err != nil {
			talking.logger.Errorf("recorder error: %v", err)
		}
	}
}

func (talking *genericRequestor) handleRecordAssistantAudio(ctx context.Context, vl internal_type.RecordAssistantAudioPacket) {
	if talking.recorder != nil {
		if err := talking.recorder.Record(ctx, vl); err != nil {
			talking.logger.Errorf("recorder error: %v", err)
		}
	}
}

func (talking *genericRequestor) handleSaveMessage(ctx context.Context, vl internal_type.MessageCreatePacket) {
	if err := talking.onAddMessage(ctx, vl); err != nil {
		talking.logger.Errorf("Error in onAddMessage: %v", err)
	}
}

func (talking *genericRequestor) handleConversationMetric(ctx context.Context, vl internal_type.ConversationMetricPacket) {
	if len(vl.Metrics) > 0 {
		_ = talking.Notify(ctx, &protos.ConversationMetric{
			AssistantConversationId: talking.Conversation().Id,
			Metrics:                 vl.Metrics,
		})
		if talking.observer != nil {
			talking.observer.EmitMetric(ctx, vl.Metrics)
		}
	}
}

func (talking *genericRequestor) handleConversationMetadata(ctx context.Context, vl internal_type.ConversationMetadataPacket) {
	if len(vl.Metadata) > 0 {
		for _, item := range vl.Metadata {
			if item == nil {
				continue
			}
			talking.metadata[item.Key] = item.Value
		}
		if err := talking.onAddMetadata(ctx, vl.Metadata...); err != nil {
			talking.logger.Errorf("Error in onAddMetadata: %v", err)
		}
	}
}

func (talking *genericRequestor) handleAssistantMessageMetric(ctx context.Context, vl internal_type.AssistantMessageMetricPacket) {
	if len(vl.Metrics) > 0 {
		_ = talking.Notify(ctx, &protos.ConversationMetric{
			AssistantConversationId: talking.Conversation().Id,
			Metrics:                 vl.Metrics,
		})
		if err := talking.onAddMessageMetric(ctx, "assistant", vl.ContextID, vl.Metrics); err != nil {
			talking.logger.Errorf("Error in onMessageMetric: %v", err)
		}
		if talking.observer != nil {
			talking.observer.MetricCollectors().Collect(ctx, observe.MessageMetricRecord{
				MessageID:      vl.ContextID,
				ConversationID: fmt.Sprintf("%d", talking.Conversation().Id),
				Metrics:        vl.Metrics,
				Time:           time.Now(),
			})
		}
	}
}

func (talking *genericRequestor) handleUserMessageMetric(ctx context.Context, vl internal_type.UserMessageMetricPacket) {
	if len(vl.Metrics) > 0 {
		_ = talking.Notify(ctx, &protos.ConversationMetric{
			AssistantConversationId: talking.Conversation().Id,
			Metrics:                 vl.Metrics,
		})
		if vl.ContextID == "" {
			vl.ContextID = talking.GetID()
		}
		if err := talking.onAddMessageMetric(ctx, "user", vl.ContextID, vl.Metrics); err != nil {
			talking.logger.Errorf("Error in onMessageMetric: %v", err)
		}
		if talking.observer != nil {
			talking.observer.MetricCollectors().Collect(ctx, observe.MessageMetricRecord{
				MessageID:      vl.ContextID,
				ConversationID: fmt.Sprintf("%d", talking.Conversation().Id),
				Metrics:        vl.Metrics,
				Time:           time.Now(),
			})
		}
	}
}

func (talking *genericRequestor) handleUserMessageMetadata(ctx context.Context, vl internal_type.UserMessageMetadataPacket) {
	if len(vl.Metadata) > 0 {
		_ = talking.Notify(ctx, &protos.ConversationMetadata{
			AssistantConversationId: talking.Conversation().Id,
			Metadata:                vl.Metadata,
		})
		if vl.ContextID == "" {
			vl.ContextID = talking.GetID()
		}
		if err := talking.onAddMessageMetadata(ctx, "user", vl.ContextID, vl.Metadata); err != nil {
			talking.logger.Errorf("Error in onAddMessageMetadata: %v", err)
		}
	}
}

func (talking *genericRequestor) handleAssistantMessageMetadata(ctx context.Context, vl internal_type.AssistantMessageMetadataPacket) {
	if len(vl.Metadata) > 0 {
		_ = talking.Notify(ctx, &protos.ConversationMetadata{
			AssistantConversationId: talking.Conversation().Id,
			Metadata:                vl.Metadata,
		})
		if vl.ContextID == "" {
			vl.ContextID = talking.GetID()
		}
		if err := talking.onAddMessageMetadata(ctx, "assistant", vl.ContextID, vl.Metadata); err != nil {
			talking.logger.Errorf("Error in onAddMessageMetadata: %v", err)
		}
	}
}

func (talking *genericRequestor) handleToolLogCreate(ctx context.Context, vl internal_type.ToolLogCreatePacket) {
	if err := talking.CreateToolLog(ctx, vl.ContextID, vl.ToolID, vl.Name, type_enums.RECORD_IN_PROGRESS, vl.Request); err != nil {
		talking.logger.Errorf("error logging tool call start: %v", err)
	}
}

func (talking *genericRequestor) handleToolLogUpdate(ctx context.Context, vl internal_type.ToolLogUpdatePacket) {
	if err := talking.UpdateToolLog(ctx, vl.ToolID, type_enums.RECORD_COMPLETE, vl.Response); err != nil {
		talking.logger.Errorf("error logging tool call result: %v", err)
	}
}

func (talking *genericRequestor) handleWebhookLogCreate(ctx context.Context, vl internal_type.WebhookLogCreatePacket) {
	if err := talking.CreateWebhookLog(
		ctx,
		vl.WebhookID,
		vl.HTTPURL,
		vl.HTTPMethod,
		vl.Event,
		vl.ResponseStatus,
		vl.TimeTaken,
		vl.RetryCount,
		vl.Status,
		vl.RequestPayload,
		vl.ResponsePayload,
	); err != nil {
		talking.logger.Errorf("error logging webhook execution: %v", err)
	}
}

func (talking *genericRequestor) handleConversationEvent(ctx context.Context, vl internal_type.ConversationEventPacket) {
	contextID := vl.ContextID
	if contextID == "" {
		contextID = talking.GetID()
	}
	if vl.Time.IsZero() {
		vl.Time = time.Now()
	}
	_ = talking.Notify(ctx, &protos.ConversationEvent{
		Id:   contextID,
		Name: vl.Name,
		Data: vl.Data,
		Time: timestamppb.New(vl.Time),
	})
	if talking.observer != nil {
		talking.observer.EventCollectors().Collect(ctx, observe.EventRecord{
			ConversationID: talking.observer.Meta().AssistantConversationID,
			MessageID:      contextID,
			Name:           vl.Name,
			Data:           vl.Data,
			Time:           vl.Time,
		})
	}
}
