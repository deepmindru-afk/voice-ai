// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.

package internal_openai_common

import (
	"errors"
	"fmt"

	"github.com/openai/openai-go/v3/responses"

	type_enums "github.com/rapidaai/pkg/types/enums"
	protos "github.com/rapidaai/protos"
)

const APIKey = "key"

func ResolveAPIKey(credential *protos.Credential) (string, error) {
	if credential == nil || credential.GetValue() == nil {
		return "", errors.New("unable to resolve the credential")
	}
	credentialValue := credential.GetValue().AsMap()
	raw, ok := credentialValue[APIKey]
	if !ok {
		return "", errors.New("unable to resolve the credential")
	}
	apiKey, ok := raw.(string)
	if !ok || apiKey == "" {
		return "", errors.New("unable to resolve the credential")
	}
	return apiKey, nil
}

func ResponseUsageMetrics(usages responses.ResponseUsage) []*protos.Metric {
	metrics := make([]*protos.Metric, 0, 3)
	metrics = append(metrics, &protos.Metric{
		Name:        type_enums.OUTPUT_TOKEN.String(),
		Value:       fmt.Sprintf("%d", usages.OutputTokens),
		Description: "Input token",
	})
	metrics = append(metrics, &protos.Metric{
		Name:        type_enums.INPUT_TOKEN.String(),
		Value:       fmt.Sprintf("%d", usages.InputTokens),
		Description: "Output Token",
	})
	metrics = append(metrics, &protos.Metric{
		Name:        type_enums.TOTAL_TOKEN.String(),
		Value:       fmt.Sprintf("%d", usages.TotalTokens),
		Description: "Total Token",
	})
	return metrics
}
