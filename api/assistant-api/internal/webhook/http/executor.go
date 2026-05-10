// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.

package internal_webhook_http

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	internal_assistant_entity "github.com/rapidaai/api/assistant-api/internal/entity/assistants"
	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	"github.com/rapidaai/pkg/clients/rest"
	"github.com/rapidaai/pkg/commons"
	type_enums "github.com/rapidaai/pkg/types/enums"
	"github.com/rapidaai/pkg/utils"
)

type runtimeExecutor struct {
	logger   commons.Logger
	callback internal_type.Callback
	webhook  *internal_assistant_entity.AssistantWebhook
}

// NewExecutor creates a fully wired HTTP webhook executor.
func NewExecutor(logger commons.Logger, _ context.Context,
	webhook *internal_assistant_entity.AssistantWebhook,
	callback internal_type.Callback,
	_ internal_type.InternalCaller) (internal_type.WebhookExecutor, error) {
	return &runtimeExecutor{
		logger:   logger,
		callback: callback,
		webhook:  webhook,
	}, nil
}

func (e *runtimeExecutor) Name() string {
	return fmt.Sprintf("webhook-http-%s", e.webhook.Id)
}

func (e *runtimeExecutor) Options() utils.Option {
	return e.webhook.GetOptions()
}

// Execute runs webhook dispatch for packet event.
func (e *runtimeExecutor) Execute(ctx context.Context, packet internal_type.ExecuteWebhookPacket) error {
	// method := strings.ToUpper()
	client := rest.NewRestClientWithConfig(e.webhook.GetUrl(), e.webhook.GetHeaders(), e.webhook.GetTimeoutSecond())
	startTime := time.Now()
	requestPayload := e.createRequestPayload(e.webhook.GetUrl(), e.webhook.GetMethod(), e.webhook.GetHeaders(), e.webhook.GetTimeoutSecond()*1000, packet.Arguments)
	for retryCount := uint32(0); retryCount <= e.webhook.GetMaxRetryCount(); retryCount++ {
		switch e.webhook.GetMethod() {
		case "POST":
			response, err := client.Post(ctx, "", packet.Arguments, e.webhook.GetHeaders())
			if err != nil {
				e.logger.Warnw("Webhook execution failed", "url", e.webhook.GetUrl(), "error", err)
				errorMessage := err.Error()
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, 0, &errorMessage, requestPayload, nil)
				if retryCount < e.webhook.GetMaxRetryCount() {
					time.Sleep(2 * time.Second)
				}
				continue
			}
			responsePayload, _ := response.ToJSON()
			isRetryable := slices.Contains(e.webhook.GetRetryStatusCode(), strconv.Itoa(response.StatusCode))
			if isRetryable {
				errorMessage := fmt.Sprintf("webhook: retryable status %d", response.StatusCode)
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, int64(response.StatusCode), &errorMessage, requestPayload, responsePayload)
				if retryCount < e.webhook.GetMaxRetryCount() {
					time.Sleep(2 * time.Second)
				}
				continue
			}
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				errorMessage := fmt.Sprintf("webhook: endpoint returned status %d", response.StatusCode)
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, int64(response.StatusCode), &errorMessage, requestPayload, responsePayload)
				return nil
			}
			e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_COMPLETE, int64(response.StatusCode), nil, requestPayload, responsePayload)
			return nil
		case "PUT":
			response, err := client.Put(ctx, "", packet.Arguments, e.webhook.GetHeaders())
			if err != nil {
				e.logger.Warnw("Webhook execution failed", "url", e.webhook.GetUrl(), "error", err)
				errorMessage := err.Error()
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, 0, &errorMessage, requestPayload, nil)
				if retryCount < e.webhook.GetMaxRetryCount() {
					time.Sleep(2 * time.Second)
				}
				continue
			}
			responsePayload, _ := response.ToJSON()
			isRetryable := slices.Contains(e.webhook.GetRetryStatusCode(), strconv.Itoa(response.StatusCode))
			if isRetryable {
				errorMessage := fmt.Sprintf("webhook: retryable status %d", response.StatusCode)
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, int64(response.StatusCode), &errorMessage, requestPayload, responsePayload)
				if retryCount < e.webhook.GetMaxRetryCount() {
					time.Sleep(2 * time.Second)
				}
				continue
			}
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				errorMessage := fmt.Sprintf("webhook: endpoint returned status %d", response.StatusCode)
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, int64(response.StatusCode), &errorMessage, requestPayload, responsePayload)
				return nil
			}
			e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_COMPLETE, int64(response.StatusCode), nil, requestPayload, responsePayload)
			return nil
		case "PATCH":
			response, err := client.Patch(ctx, "", packet.Arguments, e.webhook.GetHeaders())
			if err != nil {
				e.logger.Warnw("Webhook execution failed", "url", e.webhook.GetUrl(), "error", err)
				errorMessage := err.Error()
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, 0, &errorMessage, requestPayload, nil)
				if retryCount < e.webhook.GetMaxRetryCount() {
					time.Sleep(2 * time.Second)
				}
				continue
			}
			responsePayload, _ := response.ToJSON()
			isRetryable := slices.Contains(e.webhook.GetRetryStatusCode(), strconv.Itoa(response.StatusCode))
			if isRetryable {
				errorMessage := fmt.Sprintf("webhook: retryable status %d", response.StatusCode)
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, int64(response.StatusCode), &errorMessage, requestPayload, responsePayload)
				if retryCount < e.webhook.GetMaxRetryCount() {
					time.Sleep(2 * time.Second)
				}
				continue
			}
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				errorMessage := fmt.Sprintf("webhook: endpoint returned status %d", response.StatusCode)
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, int64(response.StatusCode), &errorMessage, requestPayload, responsePayload)
				return nil
			}
			e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_COMPLETE, int64(response.StatusCode), nil, requestPayload, responsePayload)
			return nil
		default:
			response, err := client.Get(ctx, "", packet.Arguments, e.webhook.GetHeaders())
			if err != nil {
				e.logger.Warnw("Webhook execution failed", "url", e.webhook.GetUrl(), "error", err)
				errorMessage := err.Error()
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, 0, &errorMessage, requestPayload, nil)
				if retryCount < e.webhook.GetMaxRetryCount() {
					time.Sleep(2 * time.Second)
				}
				continue
			}
			responsePayload, _ := response.ToJSON()
			isRetryable := slices.Contains(e.webhook.GetRetryStatusCode(), strconv.Itoa(response.StatusCode))
			if isRetryable {
				errorMessage := fmt.Sprintf("webhook: retryable status %d", response.StatusCode)
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, int64(response.StatusCode), &errorMessage, requestPayload, responsePayload)
				if retryCount < e.webhook.GetMaxRetryCount() {
					time.Sleep(2 * time.Second)
				}
				continue
			}
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				errorMessage := fmt.Sprintf("webhook: endpoint returned status %d", response.StatusCode)
				e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_FAILED, int64(response.StatusCode), &errorMessage, requestPayload, responsePayload)
				return nil
			}
			e.onCreateLog(ctx, packet, e.webhook.GetMethod(), startTime, retryCount, type_enums.RECORD_COMPLETE, int64(response.StatusCode), nil, requestPayload, responsePayload)
			return nil
		}
	}
	return nil
}

func (e *runtimeExecutor) createRequestPayload(url, method string, headers map[string]string, timeoutMs uint32, body map[string]interface{}) []byte {
	payload := map[string]interface{}{
		"url":        url,
		"method":     method,
		"headers":    headers,
		"timeout_ms": timeoutMs,
		"body":       body,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		e.logger.Warnw("Failed to serialize webhook request payload snapshot", "error", err)
		return nil
	}
	return data
}

func (e *runtimeExecutor) onCreateLog(
	ctx context.Context,
	packet internal_type.ExecuteWebhookPacket,
	method string,
	startTime time.Time,
	retryCount uint32,
	status type_enums.RecordState,
	responseStatus int64,
	errorMessage *string,
	requestPayload []byte,
	responsePayload []byte,
) {
	sourceRefID := e.webhook.Id
	if err := e.callback.OnPacket(ctx, internal_type.HTTPLogCreatePacket{
		ContextID:       packet.ContextID,
		Source:          "webhook",
		SourceRefID:     sourceRefID,
		SourceEvent:     packet.Event.Get(),
		HTTPURL:         e.webhook.GetUrl(),
		HTTPMethod:      method,
		ResponseStatus:  responseStatus,
		TimeTaken:       int64(time.Since(startTime)),
		RetryCount:      retryCount,
		Status:          status,
		ErrorMessage:    errorMessage,
		RequestPayload:  requestPayload,
		ResponsePayload: responsePayload,
	}); err != nil {
		e.logger.Warnw("Failed to enqueue webhook log", "error", err)
	}
}

// Close releases executor dependencies.
func (e *runtimeExecutor) Close(_ context.Context) error {
	e.callback = nil
	return nil
}
