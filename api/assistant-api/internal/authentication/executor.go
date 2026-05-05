// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.

package authentication

import (
	"context"
	"fmt"
	"time"

	internal_type "github.com/rapidaai/api/assistant-api/internal/type"
	"github.com/rapidaai/pkg/clients/rest"
	"github.com/rapidaai/pkg/commons"
)

const (
	OptionHTTPURLKey     = "http_url"
	OptionHTTPMethodKey  = "http_method"
	OptionHTTPHeadersKey = "http_headers"

	FailBehaviorBlock = "block"
	FailBehaviorAllow = "allow"
)

// Result carries the outcome of an authentication attempt.
type Result struct {
	Authenticated bool
	Args          map[string]interface{}
	Metadata      map[string]interface{}
	Options       map[string]interface{}
}

// Executor defines authentication runtime behavior.
type Executor interface {
	Init(ctx context.Context, communication internal_type.Communication)
	Execute(ctx context.Context, packet internal_type.ExecuteAuthenticationPacket) (*Result, error)
	Close(ctx context.Context)
}

type runtimeExecutor struct {
	logger   commons.Logger
	onPacket func(ctx context.Context, pkts ...internal_type.Packet) error
}

// NewExecutor creates an authentication executor.
func NewExecutor(logger commons.Logger) Executor {
	return &runtimeExecutor{logger: logger}
}

// Init wires live communication dependencies required by executor.
func (e *runtimeExecutor) Init(_ context.Context, communication internal_type.Communication) {
	e.onPacket = communication.OnPacket
}

// Execute runs authentication against the configured endpoint.
func (e *runtimeExecutor) Execute(ctx context.Context, packet internal_type.ExecuteAuthenticationPacket) (*Result, error) {
	auth := packet.Authentication
	opts := auth.GetOptions()

	url, err := opts.GetString(OptionHTTPURLKey)
	if err != nil || url == "" {
		return nil, fmt.Errorf("authentication: missing %s", OptionHTTPURLKey)
	}

	method := "POST"
	if m, err := opts.GetString(OptionHTTPMethodKey); err == nil && m != "" {
		method = m
	}

	headers := map[string]string{}
	if h, err := opts.GetStringMap(OptionHTTPHeadersKey); err == nil {
		headers = h
	}

	timeout := auth.TimeoutMs
	if timeout == 0 {
		timeout = 5000
	}

	client := rest.NewRestClientWithConfig(url, headers, uint32(timeout/1000))

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	response, err := e.send(callCtx, client, method, packet.Arguments, headers)
	if err != nil {
		if auth.FailBehavior == FailBehaviorAllow {
			e.logger.Warnw("authentication failed, allowing due to fail_behavior=allow", "url", url, "error", err)
			return &Result{Authenticated: false}, nil
		}
		return nil, fmt.Errorf("authentication: request failed: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if auth.FailBehavior == FailBehaviorAllow {
			e.logger.Warnw("authentication returned non-2xx, allowing due to fail_behavior=allow",
				"url", url, "status", response.StatusCode)
			return &Result{Authenticated: false}, nil
		}
		return nil, fmt.Errorf("authentication: endpoint returned status %d", response.StatusCode)
	}

	result := &Result{Authenticated: true}
	if parsed, err := response.ToMap(); err == nil {
		if args, ok := parsed["args"].(map[string]interface{}); ok {
			result.Args = args
		}
		if metadata, ok := parsed["metadata"].(map[string]interface{}); ok {
			result.Metadata = metadata
		}
		if options, ok := parsed["options"].(map[string]interface{}); ok {
			result.Options = options
		}
	}

	return result, nil
}

// Close releases executor dependencies.
func (e *runtimeExecutor) Close(_ context.Context) {
	e.onPacket = nil
}

func (e *runtimeExecutor) send(ctx context.Context, client *rest.RestClient, method string, body map[string]interface{}, headers map[string]string) (*rest.APIResponse, error) {
	switch method {
	case "POST":
		return client.Post(ctx, "", body, headers)
	case "PUT":
		return client.Put(ctx, "", body, headers)
	case "PATCH":
		return client.Patch(ctx, "", body, headers)
	default:
		return client.Get(ctx, "", body, headers)
	}
}
