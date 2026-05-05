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

	internal_audio_recorder "github.com/rapidaai/api/assistant-api/internal/audio/recorder"
	internal_conversation_entity "github.com/rapidaai/api/assistant-api/internal/entity/conversations"
	observe "github.com/rapidaai/api/assistant-api/internal/observe"
	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	"github.com/rapidaai/api/assistant-api/internal/variable"
	internal_namespace "github.com/rapidaai/api/assistant-api/internal/variable/namespace"
	"github.com/rapidaai/pkg/types"
	type_enums "github.com/rapidaai/pkg/types/enums"
	"github.com/rapidaai/pkg/utils"
	"github.com/rapidaai/protos"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// =============================================================================
// Handlers
// =============================================================================

func (r *genericRequestor) handleInitAssistant(ctx context.Context, pkt internal_type.InitAssistantPacket) {
	assistant, err := r.GetAssistant(ctx, r.Auth(), pkt.Config.Assistant.AssistantId, pkt.Config.Assistant.Version)
	if err != nil {
		r.logger.Errorf("failed to retrieve assistant configuration: %+v", err)
		r.OnPacket(ctx, internal_type.InitializationErrorPacket{ContextID: pkt.ContextID, Error: err})
		return
	}
	r.assistant = assistant
	r.authenticationExecutor.Init(ctx, r)
	r.OnPacket(ctx, internal_type.InitConversationPacket{ContextID: pkt.ContextID, Config: pkt.Config})
}

func (r *genericRequestor) handleInitConversation(ctx context.Context, pkt internal_type.InitConversationPacket) {
	config := pkt.Config
	if conversationID := config.GetAssistantConversationId(); conversationID > 0 {
		conversation, err := r.ResumeConversation(ctx, r.assistant, config)
		if err != nil {
			r.logger.Errorf("failed to resume conversation: %+v", err)
			r.OnPacket(ctx, internal_type.InitializationErrorPacket{ContextID: pkt.ContextID, Error: err})
			return
		}
		r.OnPacket(ctx, internal_type.ConversationEventPacket{
			Name: "session",
			Data: map[string]string{
				"type":          "resumed",
				"source":        fmt.Sprintf("%v", r.source),
				"identifier":    r.identifier(config),
				"message_count": fmt.Sprintf("%d", len(r.GetHistories())),
			},
			Time: time.Now(),
		})
		r.notifyConfiguration(ctx, config, conversation)
	} else {
		conversation, err := r.BeginConversation(ctx, r.assistant, type_enums.DIRECTION_INBOUND, config)
		if err != nil {
			r.logger.Errorf("failed to begin conversation: %+v", err)
			r.OnPacket(ctx, internal_type.InitializationErrorPacket{ContextID: pkt.ContextID, Error: err})
			return
		}
		r.OnPacket(ctx, internal_type.ConversationEventPacket{
			Name: observe.ComponentSession,
			Data: map[string]string{
				observe.DataType: observe.EventConnected,
				"source":         fmt.Sprintf("%v", r.source),
				"is_new":         "true",
				"identifier":     r.identifier(config),
			},
			Time: time.Now(),
		})
		r.notifyConfiguration(ctx, config, conversation)
	}
	r.OnPacket(ctx, internal_type.InitServicePacket{ContextID: pkt.ContextID, Config: config})
}

func (r *genericRequestor) handleInitService(ctx context.Context, pkt internal_type.InitServicePacket) {
	config := pkt.Config

	utils.Go(ctx, func() { r.initializeCollectors(ctx) })

	utils.Go(ctx, func() {
		rc, err := internal_audio_recorder.GetRecorder(r.logger)
		if err != nil {
			r.logger.Tracef(ctx, "failed to initialize audio recorder: %+v", err)
			return
		}
		r.recorder = rc
		r.recorder.Start()
		r.OnPacket(ctx, internal_type.ConversationEventPacket{
			Name: observe.ComponentRecording,
			Data: map[string]string{observe.DataType: observe.EventRecordingStarted},
			Time: time.Now(),
		})
	})

	if err := r.initializeInputNormalizer(ctx, config); err != nil {
		r.logger.Tracef(ctx, "failed to initialize input normalizer: %+v", err)
	}
	if err := r.initializeOutputNormalizer(ctx, config); err != nil {
		r.logger.Errorf("failed to initialize output normalizer: %v", err)
	}

	utils.Go(ctx, func() {
		if err := r.initializeEndOfSpeech(ctx); err != nil {
			r.logger.Tracef(ctx, "failed to initialize end of speech: %+v", err)
		}
	})

	// Emit the initial "in progress" status metric via the normal packet dispatch
	// path. This avoids a data race: initializeCollectors sets r.observer in a
	// separate goroutine, so accessing it directly here would be racy.
	// ConversationMetricPacket flows through lowCh → handleConversationMetric,
	// which already nil-checks r.observer before exporting.
	utils.Go(ctx, func() {
		metrics := []*protos.Metric{{
			Name:        type_enums.CONVERSATION_STATUS.String(),
			Value:       type_enums.CONVERSATION_IN_PROGRESS.String(),
			Description: "Conversation is currently in progress",
		}}
		r.onAddMetrics(ctx, metrics...)
		r.OnPacket(ctx, internal_type.ConversationMetricPacket{
			ContextID: r.Conversation().Id,
			Metrics:   metrics,
		})
	})

	utils.Go(ctx, func() { r.storeClientInformation(ctx) })

	r.OnPacket(ctx, internal_type.InitAuthenticatePacket{ContextID: pkt.ContextID, Config: config})
}

func (r *genericRequestor) handleInitAuthenticate(ctx context.Context, pkt internal_type.InitAuthenticatePacket) {
	if r.assistant.AssistantAuthentication != nil {
		packet := internal_type.ExecuteAuthenticationPacket{
			ContextID:      pkt.ContextID,
			Authentication: r.assistant.AssistantAuthentication,
			Arguments:      r.buildAuthBody(),
		}
		result, err := r.authenticationExecutor.Execute(ctx, packet)
		if err != nil {
			r.OnPacket(ctx, internal_type.InitializationErrorPacket{ContextID: pkt.ContextID, Error: err})
			return
		}
		if result != nil && result.Authenticated {
			if result.Args != nil {
				r.args = utils.MergeMaps(r.args, result.Args)
			}
			if result.Metadata != nil {
				r.metadata = utils.MergeMaps(r.metadata, result.Metadata)
			}
			if result.Options != nil {
				r.options = utils.MergeMaps(r.options, result.Options)
			}
		}
	}
	r.OnPacket(ctx, internal_type.InitAudioPacket{ContextID: pkt.ContextID, Config: pkt.Config})
}

func (r *genericRequestor) handleInitAudio(ctx context.Context, pkt internal_type.InitAudioPacket) {
	config := pkt.Config

	if err := r.assistantExecutor.Initialize(ctx, r, config); err != nil {
		r.logger.Tracef(ctx, "failed to initialize executor: %+v", err)
		r.OnPacket(ctx, internal_type.InitializationErrorPacket{ContextID: pkt.ContextID, Error: err})
		return
	}

	switch config.StreamMode {
	case protos.StreamMode_STREAM_MODE_TEXT:
		r.SwitchMode(type_enums.TextMode)
	case protos.StreamMode_STREAM_MODE_AUDIO:
		if err := r.initializeTextToSpeech(ctx); err != nil {
			r.logger.Errorf("failed to initialize text-to-speech: %v", err)
			r.OnPacket(ctx, internal_type.InitializationErrorPacket{ContextID: pkt.ContextID, Error: err})
			return
		}
		r.SwitchMode(type_enums.AudioMode)
		if err := r.initializeSpeechToText(ctx); err != nil {
			r.logger.Errorf("failed to initialize speech-to-text: %v", err)
			r.OnPacket(ctx, internal_type.InitializationErrorPacket{ContextID: pkt.ContextID, Error: err})
			return
		}
	}

	isNew := config.GetAssistantConversationId() == 0
	if isNew {
		r.OnPacket(ctx, internal_type.WebhookStartPacket{ContextID: pkt.ContextID, Event: utils.ConversationBegin})
	} else {
		r.OnPacket(ctx, internal_type.WebhookStartPacket{ContextID: pkt.ContextID, Event: utils.ConversationResume})
	}

	r.OnPacket(ctx, internal_type.InitBehaviorPacket{ContextID: pkt.ContextID, Config: config})
}

func (r *genericRequestor) handleInitBehavior(ctx context.Context, _ internal_type.InitBehaviorPacket) {
	r.initializeBehavior(ctx)
}

// =============================================================================
// Helpers
// =============================================================================

func (r *genericRequestor) buildAuthBody() map[string]interface{} {
	source := variable.NewCommunicationSource(r)
	registry := internal_namespace.NewDefaultRegistry()
	return registry.Expand(source, variable.ResolveContext{})
}

func (r *genericRequestor) notifyConfiguration(ctx context.Context, config *protos.ConversationInitialization, conversation *internal_conversation_entity.AssistantConversation) {
	options := config.GetOptions()
	mergedOptions := map[string]interface{}{}
	if base, err := utils.AnyMapToInterfaceMap(config.GetOptions()); err == nil {
		mergedOptions = base
	}
	if outputAudio, err := r.GetTextToSpeechTransformer(); err == nil && outputAudio != nil {
		outputOpts := outputAudio.GetOptions()
		if ambient, err := outputOpts.GetString("speaker.ambient"); err == nil && ambient != "" {
			mergedOptions["speaker.ambient"] = ambient
		}
		if volume, err := outputOpts.GetString("speaker.ambient_volume"); err == nil && volume != "" {
			mergedOptions["speaker.ambient_volume"] = volume
		} else if volumeNum, err := outputOpts.GetUint64("speaker.ambient_volume"); err == nil {
			mergedOptions["speaker.ambient_volume"] = volumeNum
		}
	}
	if len(mergedOptions) > 0 {
		if anyMap, err := utils.InterfaceMapToAnyMap(mergedOptions); err == nil {
			options = anyMap
		}
	}
	if err := r.Notify(ctx, &protos.ConversationInitialization{
		AssistantConversationId: conversation.Id,
		Assistant: &protos.AssistantDefinition{
			AssistantId: r.assistant.Id,
			Version:     utils.GetVersionString(r.assistant.AssistantProviderId),
		},
		Args:         config.GetArgs(),
		Metadata:     config.GetMetadata(),
		Options:      options,
		StreamMode:   config.GetStreamMode(),
		UserIdentity: config.GetUserIdentity(),
		Time:         timestamppb.Now(),
	}); err != nil {
		r.logger.Errorf("failed to send configuration notification: %v", err)
	}
}


func (r *genericRequestor) storeClientInformation(ctx context.Context) {
	clientInfo := types.GetClientInfoFromGrpcContext(ctx)
	if clientInfo == nil {
		return
	}
	flat := map[string]interface{}{}
	if clientInfo.Timezone != "" {
		flat["client.timezone"] = clientInfo.Timezone
	}
	if clientInfo.Platform != "" {
		flat["client.platform"] = clientInfo.Platform
	}
	if clientInfo.Language != "" {
		flat["client.language"] = clientInfo.Language
	}
	if clientInfo.UserAgent != "" {
		flat["client.user_agent"] = clientInfo.UserAgent
	}
	if clientInfo.Referrer != "" {
		flat["client.referrer"] = clientInfo.Referrer
	}
	if clientInfo.ConnectionType != "" {
		flat["client.connection_type"] = clientInfo.ConnectionType
	}
	if clientInfo.Latitude != 0 || clientInfo.Longitude != 0 {
		flat["client.latitude"] = fmt.Sprintf("%f", clientInfo.Latitude)
		flat["client.longitude"] = fmt.Sprintf("%f", clientInfo.Longitude)
	}
	r.onSetMetadata(ctx, r.Auth(), flat)
}
