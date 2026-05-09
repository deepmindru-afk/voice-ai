package internal_custom_llm_openai_responses

import (
	"context"
	"fmt"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"google.golang.org/protobuf/types/known/anypb"

	internal_custom_llm_common "github.com/rapidaai/api/integration-api/internal/caller/custom_llm/common"
	internal_caller_metrics "github.com/rapidaai/api/integration-api/internal/caller/metrics"
	internal_callers "github.com/rapidaai/api/integration-api/internal/type"
	"github.com/rapidaai/pkg/commons"
	type_enums "github.com/rapidaai/pkg/types/enums"
	"github.com/rapidaai/pkg/utils"
	"github.com/rapidaai/protos"
)

type adapter struct {
	logger commons.Logger
	client *openai.Client
}

func New(
	dependencies internal_custom_llm_common.AdapterDependencies,
) (internal_custom_llm_common.Adapter, error) {
	if dependencies.OpenAIClient == nil {
		return nil, fmt.Errorf(
			"custom-llm: %s adapter missing OpenAI client",
			internal_custom_llm_common.CompatibilityOpenAIResponses,
		)
	}
	return &adapter{
		logger: dependencies.Logger,
		client: dependencies.OpenAIClient,
	}, nil
}

func (a *adapter) GetChatCompletion(
	ctx context.Context,
	allMessages []*protos.Message,
	options *internal_callers.ChatCompletionOptions,
) (*protos.Message, []*protos.Metric, error) {
	metrics := internal_caller_metrics.NewMetricBuilder(options.RequestId)
	metrics.OnStart()

	request := a.buildResponseParams(options)
	request.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: a.buildHistory(allMessages),
	}
	options.PreHook(utils.ToJson(request))

	resp, err := a.client.Responses.New(ctx, request)
	if err != nil {
		a.logger.Errorf("custom-llm openai_responses: request failed: %v", err)
		failure := metrics.OnFailure().Build()
		options.PostHook(map[string]interface{}{
			"error":  err,
			"result": resp,
		}, failure)
		return nil, failure, err
	}

	assistantMessage := &protos.AssistantMessage{
		Contents:  make([]string, 0),
		ToolCalls: make([]*protos.ToolCall, 0),
	}
	if outputText := resp.OutputText(); outputText != "" {
		assistantMessage.Contents = append(assistantMessage.Contents, outputText)
	}
	for _, outputItem := range resp.Output {
		if outputItem.Type != "function_call" {
			continue
		}
		functionCall := outputItem.AsFunctionCall()
		callID := functionCall.CallID
		if callID == "" {
			callID = functionCall.ID
		}
		assistantMessage.ToolCalls = append(assistantMessage.ToolCalls, &protos.ToolCall{
			Id:   callID,
			Type: "function",
			Function: &protos.FunctionCall{
				Name:      functionCall.Name,
				Arguments: functionCall.Arguments,
			},
		})
	}

	protoMessage := &protos.Message{
		Role: internal_custom_llm_common.ChatRoleAssistant,
		Message: &protos.Message_Assistant{
			Assistant: assistantMessage,
		},
	}

	metrics.OnAddMetrics(a.buildResponseUsageMetrics(resp.Usage)...)
	success := metrics.OnSuccess().Build()
	options.PostHook(map[string]interface{}{"result": resp}, success)
	return protoMessage, success, nil
}

func (a *adapter) StreamChatCompletion(
	ctx context.Context,
	allMessages []*protos.Message,
	options *internal_callers.ChatCompletionOptions,
	onStream func(string, *protos.Message) error,
	onMetrics func(string, *protos.Message, []*protos.Metric) error,
	onError func(string, error),
) error {
	start := time.Now()
	metrics := internal_caller_metrics.NewMetricBuilder(options.RequestId)
	metrics.OnStart()
	var firstTokenAt *time.Time

	request := a.buildResponseParams(options)
	request.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: a.buildHistory(allMessages),
	}
	options.PreHook(utils.ToJson(request))

	stream := a.client.Responses.NewStreaming(ctx, request)
	if stream.Err() != nil {
		err := stream.Err()
		a.logger.Errorf("custom-llm openai_responses: stream init failed: %v", err)
		failure := metrics.OnFailure().Build()
		options.PostHook(map[string]interface{}{
			"error":  err,
			"result": utils.ToJson(stream),
		}, failure)
		if onError != nil {
			onError(options.Request.GetRequestId(), err)
		}
		return err
	}
	defer stream.Close()

	assistantMessage := &protos.AssistantMessage{
		Contents:  make([]string, 0),
		ToolCalls: make([]*protos.ToolCall, 0),
	}
	var contentBuilder strings.Builder
	hasToolCalls := false
	var finalResponse *responses.Response

	for stream.Next() {
		event := stream.Current()
		switch typed := event.AsAny().(type) {
		case responses.ResponseTextDeltaEvent:
			if typed.Delta == "" {
				continue
			}
			contentBuilder.WriteString(typed.Delta)
			if hasToolCalls {
				continue
			}
			if firstTokenAt == nil {
				now := time.Now()
				firstTokenAt = &now
			}
			if onStream != nil {
				tokenMessage := &protos.Message{
					Role: internal_custom_llm_common.ChatRoleAssistant,
					Message: &protos.Message_Assistant{
						Assistant: &protos.AssistantMessage{Contents: []string{typed.Delta}},
					},
				}
				if err := onStream(options.Request.GetRequestId(), tokenMessage); err != nil {
					a.logger.Warnf("custom-llm openai_responses: onStream error: %v", err)
				}
			}
		case responses.ResponseFunctionCallArgumentsDeltaEvent:
			hasToolCalls = true
		case responses.ResponseFunctionCallArgumentsDoneEvent:
			hasToolCalls = true
		case responses.ResponseOutputItemAddedEvent:
			if typed.Item.Type == "function_call" {
				hasToolCalls = true
			}
		case responses.ResponseOutputItemDoneEvent:
			if typed.Item.Type == "function_call" {
				hasToolCalls = true
			}
		case responses.ResponseCompletedEvent:
			finalResponse = &typed.Response
			if a.containsFunctionCall(typed.Response.Output) {
				hasToolCalls = true
			}
		}
	}

	if stream.Err() != nil {
		err := stream.Err()
		a.logger.Errorf("custom-llm openai_responses: stream read failed: %v", err)
		failure := metrics.OnFailure().Build()
		options.PostHook(map[string]interface{}{
			"error":  err,
			"result": utils.ToJson(stream),
		}, failure)
		if onError != nil {
			onError(options.Request.GetRequestId(), err)
		}
		return err
	}

	if finalResponse != nil {
		if outputText := finalResponse.OutputText(); outputText != "" {
			assistantMessage.Contents = append(assistantMessage.Contents, outputText)
		}
		for _, outputItem := range finalResponse.Output {
			if outputItem.Type != "function_call" {
				continue
			}
			functionCall := outputItem.AsFunctionCall()
			callID := functionCall.CallID
			if callID == "" {
				callID = functionCall.ID
			}
			assistantMessage.ToolCalls = append(assistantMessage.ToolCalls, &protos.ToolCall{
				Id:   callID,
				Type: "function",
				Function: &protos.FunctionCall{
					Name:      functionCall.Name,
					Arguments: functionCall.Arguments,
				},
			})
		}
		metrics.OnAddMetrics(a.buildResponseUsageMetrics(finalResponse.Usage)...)
	} else if contentBuilder.Len() > 0 {
		assistantMessage.Contents = append(assistantMessage.Contents, contentBuilder.String())
	}

	protoMessage := &protos.Message{
		Role: internal_custom_llm_common.ChatRoleAssistant,
		Message: &protos.Message_Assistant{
			Assistant: assistantMessage,
		},
	}

	if firstTokenAt != nil {
		metrics.OnAddMetrics(&protos.Metric{
			Name:        type_enums.TIME_TO_FIRST_TOKEN.String(),
			Value:       fmt.Sprintf("%d", firstTokenAt.Sub(start)),
			Description: "Time to receive first token from LLM",
		})
	}
	success := metrics.OnSuccess().Build()
	if onMetrics != nil {
		onMetrics(options.Request.GetRequestId(), protoMessage, success)
	}
	resultPayload := utils.ToJson(stream)
	if finalResponse != nil {
		resultPayload = utils.ToJson(finalResponse)
	}
	options.PostHook(map[string]interface{}{"result": resultPayload}, success)
	return nil
}

func (a *adapter) VerifyCredential(
	ctx context.Context,
	options *internal_callers.CredentialVerifierOptions,
) (*string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	_, err := a.client.Models.List(ctx)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (a *adapter) buildResponseParams(
	opts *internal_callers.ChatCompletionOptions,
) responses.ResponseNewParams {
	request := responses.ResponseNewParams{
		Store: openai.Bool(false),
	}

	if len(opts.ToolDefinitions) > 0 {
		tools := make([]responses.ToolUnionParam, 0, len(opts.ToolDefinitions))
		for _, toolDefinition := range opts.ToolDefinitions {
			if toolDefinition.Type != "function" || toolDefinition.Function == nil {
				continue
			}
			function := toolDefinition.Function
			functionTool := responses.FunctionToolParam{
				Name:   function.Name,
				Strict: openai.Bool(false),
			}
			if function.Description != "" {
				functionTool.Description = openai.String(function.Description)
			}
			if function.Parameters != nil {
				functionTool.Parameters = function.Parameters.ToMap()
			} else {
				functionTool.Parameters = map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				}
			}
			tools = append(tools, responses.ToolUnionParam{OfFunction: &functionTool})
		}
		request.Tools = tools
	}

	directModelParameters := make(map[string]*anypb.Any)
	nestedModelParameters := make(map[string]*anypb.Any)

	for key, value := range opts.ModelParameter {
		if value == nil {
			continue
		}
		switch key {
		case "model.name":
			if modelName, err := utils.AnyToString(value); err == nil {
				request.Model = shared.ResponsesModel(modelName)
			}
		case "model.parameters":
			if parameterMap, err := utils.AnyToJSON(value); err == nil {
				if anyMap, err := utils.InterfaceMapToAnyMap(parameterMap); err == nil {
					for parameterKey, parameterValue := range anyMap {
						nestedModelParameters[parameterKey] = parameterValue
					}
				}
			}
		default:
			if strings.HasPrefix(key, "model.") {
				directModelParameters[strings.TrimPrefix(key, "model.")] = value
			}
		}
	}

	extraFields := map[string]interface{}{}
	a.applyResponseParameters(&request, opts, directModelParameters, extraFields)
	a.applyResponseParameters(&request, opts, nestedModelParameters, extraFields)
	if len(extraFields) > 0 {
		request.SetExtraFields(extraFields)
	}

	return request
}

func (a *adapter) applyResponseParameters(
	request *responses.ResponseNewParams,
	opts *internal_callers.ChatCompletionOptions,
	parameters map[string]*anypb.Any,
	extraFields map[string]interface{},
) {
	for rawKey, rawValue := range parameters {
		if rawValue == nil {
			continue
		}
		switch strings.ToLower(rawKey) {
		case "user":
			if user, err := utils.AnyToString(rawValue); err == nil {
				request.User = openai.String(user)
			}
		case "reasoning_effort":
			if effort, err := utils.AnyToString(rawValue); err == nil {
				request.Reasoning = shared.ReasoningParam{Effort: shared.ReasoningEffort(effort)}
			}
		case "service_tier":
			if tier, err := utils.AnyToString(rawValue); err == nil {
				request.ServiceTier = responses.ResponseNewParamsServiceTier(tier)
			}
		case "top_logprobs":
			if topLogprobs, err := utils.AnyToInt64(rawValue); err == nil {
				request.TopLogprobs = openai.Int(topLogprobs)
			}
		case "metadata":
			metadataValue, err := utils.AnyToInterface(rawValue)
			if err != nil {
				continue
			}
			metadata, err := utils.Option{rawKey: metadataValue}.GetStringMap(rawKey)
			if err != nil {
				continue
			}
			request.Metadata = shared.Metadata(metadata)
		case "temperature":
			if temperature, err := utils.AnyToFloat64(rawValue); err == nil {
				request.Temperature = openai.Float(temperature)
			}
		case "top_p":
			if topP, err := utils.AnyToFloat64(rawValue); err == nil {
				request.TopP = openai.Float(topP)
			}
		case "max_completion_tokens", "max_output_tokens", "max_tokens":
			if maxOutputTokens, err := utils.AnyToInt64(rawValue); err == nil {
				request.MaxOutputTokens = openai.Int(maxOutputTokens)
			}
		case "store":
			if store, err := utils.AnyToBool(rawValue); err == nil {
				request.Store = openai.Bool(store)
			}
		case "tool_choice":
			if len(opts.ToolDefinitions) == 0 {
				continue
			}
			choice, err := utils.AnyToString(rawValue)
			if err != nil {
				continue
			}
			switch choice {
			case "auto":
				request.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
					OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsAuto),
				}
			case "required":
				request.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
					OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsRequired),
				}
			case "none":
				request.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
					OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsNone),
				}
			}
		case "response_format":
			responseFormat, err := utils.AnyToJSON(rawValue)
			if err != nil {
				continue
			}
			formatType, ok := responseFormat["type"].(string)
			if !ok {
				continue
			}
			switch formatType {
			case "json_object":
				request.Text.Format = responses.ResponseFormatTextConfigUnionParam{
					OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
				}
			case "text":
				request.Text.Format = responses.ResponseFormatTextConfigUnionParam{
					OfText: &shared.ResponseFormatTextParam{},
				}
			case "json_schema":
				schemaData, ok := responseFormat["json_schema"].(map[string]interface{})
				if !ok {
					continue
				}
				schemaConfig := responses.ResponseFormatTextJSONSchemaConfigParam{Name: "response"}
				if name, ok := schemaData["name"].(string); ok && name != "" {
					schemaConfig.Name = name
				}
				if description, ok := schemaData["description"].(string); ok && description != "" {
					schemaConfig.Description = openai.String(description)
				}
				if strict, ok := schemaData["strict"].(bool); ok {
					schemaConfig.Strict = openai.Bool(strict)
				}
				if schema, ok := schemaData["schema"].(map[string]interface{}); ok {
					schemaConfig.Schema = schema
				} else {
					schemaConfig.Schema = map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					}
				}
				request.Text.Format = responses.ResponseFormatTextConfigUnionParam{
					OfJSONSchema: &schemaConfig,
				}
			}
		default:
			if rawParameter, err := utils.AnyToInterface(rawValue); err == nil {
				extraFields[rawKey] = rawParameter
			}
		}
	}
}

func (a *adapter) buildHistory(
	allMessages []*protos.Message,
) []responses.ResponseInputItemUnionParam {
	messageHistory := make([]responses.ResponseInputItemUnionParam, 0, len(allMessages))
	for _, message := range allMessages {
		switch message.GetRole() {
		case internal_custom_llm_common.ChatRoleUser:
			if user := message.GetUser(); user != nil {
				messageHistory = append(messageHistory, responses.ResponseInputItemParamOfMessage(
					user.GetContent(),
					responses.EasyInputMessageRoleUser,
				))
			}
		case internal_custom_llm_common.ChatRoleAssistant:
			if assistant := message.GetAssistant(); assistant != nil {
				assistantContent := strings.Join(assistant.GetContents(), "")
				if assistantContent != "" {
					messageHistory = append(messageHistory, responses.ResponseInputItemParamOfMessage(
						assistantContent,
						responses.EasyInputMessageRoleAssistant,
					))
				}
				for _, toolCall := range assistant.GetToolCalls() {
					if toolCall.GetFunction() == nil || toolCall.GetId() == "" {
						continue
					}
					messageHistory = append(messageHistory, responses.ResponseInputItemParamOfFunctionCall(
						toolCall.GetFunction().GetArguments(),
						toolCall.GetId(),
						toolCall.GetFunction().GetName(),
					))
				}
			}
		case internal_custom_llm_common.ChatRoleSystem:
			if system := message.GetSystem(); system != nil && system.GetContent() != "" {
				messageHistory = append(messageHistory, responses.ResponseInputItemParamOfMessage(
					system.GetContent(),
					responses.EasyInputMessageRoleSystem,
				))
			}
		case internal_custom_llm_common.ChatRoleTool:
			if toolMessage := message.GetTool(); toolMessage != nil {
				for _, tool := range toolMessage.GetTools() {
					if tool.GetId() == "" {
						continue
					}
					messageHistory = append(messageHistory, responses.ResponseInputItemParamOfFunctionCallOutput(
						tool.GetId(),
						tool.GetContent(),
					))
				}
			}
		}
	}
	return messageHistory
}

func (a *adapter) containsFunctionCall(outputItems []responses.ResponseOutputItemUnion) bool {
	for _, item := range outputItems {
		if item.Type == "function_call" {
			return true
		}
	}
	return false
}

func (a *adapter) buildResponseUsageMetrics(usages responses.ResponseUsage) []*protos.Metric {
	return []*protos.Metric{
		{
			Name:        type_enums.OUTPUT_TOKEN.String(),
			Value:       fmt.Sprintf("%d", usages.OutputTokens),
			Description: "LLM Output token",
		},
		{
			Name:        type_enums.INPUT_TOKEN.String(),
			Value:       fmt.Sprintf("%d", usages.InputTokens),
			Description: "LLM Input token",
		},
		{
			Name:        type_enums.TOTAL_TOKEN.String(),
			Value:       fmt.Sprintf("%d", usages.TotalTokens),
			Description: "LLM Total token",
		},
	}
}
