// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package adapter_internal

import (
	"context"

	internal_condition "github.com/rapidaai/api/assistant-api/internal/condition"
	internal_assistant_entity "github.com/rapidaai/api/assistant-api/internal/entity/assistants"
	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	"github.com/rapidaai/api/assistant-api/internal/variable"
	"github.com/rapidaai/pkg/utils"
)

func (r *genericRequestor) onAnalysisEvent(ctx context.Context, contextID string) error {
	source := variable.NewCommunicationSource(r)
	registry := variable.NewDefaultRegistry().With("event", &variable.EventNamespace{})
	for _, analysis := range r.assistant.AssistantAnalyses {
		if !r.isAnalysisAllowed(analysis, r.Conversation().Direction.String()) {
			continue
		}
		args := registry.Apply(
			analysis.GetParameters(),
			source,
			variable.ResolveContext{Event: utils.ConversationCompleted.Get()},
		)
		if err := r.OnPacket(ctx, internal_type.RunAnalysisPacket{
			ContextID:      contextID,
			Analysis:       analysis,
			Arguments:      args,
			ConversationID: r.assistantConversation.Id,
			Auth:           r.auth,
		}); err != nil {
			r.logger.Warnw("failed to enqueue analysis packet", "name", analysis.GetName(), "error", err)
		}
	}
	return r.onWebhookEvent(ctx, contextID, utils.ConversationCompleted)
}

func (r *genericRequestor) isAnalysisAllowed(analysis *internal_assistant_entity.AssistantAnalysis, direction string) bool {
	rawCondition, err := analysis.GetOptions().GetString("analysis.condition")
	if err != nil {
		return true
	}
	parsed, parseErr := internal_condition.Parse(rawCondition)
	if parseErr != nil {
		r.logger.Warnf("invalid analysis.condition for analysis %s, excluding analysis: %v", analysis.GetName(), parseErr)
		return false
	}
	allowed, evalErr := parsed.Run(
		internal_condition.ConditionValue{RuleType: internal_condition.RuleTypeSource, Value: r.GetSource().Get()},
		internal_condition.ConditionValue{RuleType: internal_condition.RuleTypeMode, Value: r.GetMode().String()},
		internal_condition.ConditionValue{RuleType: internal_condition.RuleTypeDirection, Value: direction},
	)
	if evalErr != nil {
		r.logger.Warnf("invalid analysis.condition for analysis %s, excluding analysis: %v", analysis.GetName(), evalErr)
		return false
	}
	return allowed
}
