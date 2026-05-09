package internal_custom_llm_common

import "time"

type Compatibility string

const (
	CompatibilityOpenAIChatCompletions Compatibility = "openai_chat_completions"
	CompatibilityOpenAIResponses       Compatibility = "openai_responses"
	CompatibilityAnthropicMessages     Compatibility = "anthropic_messages"
	CompatibilityGeminiGenerateContent Compatibility = "gemini_generate_content"
	CompatibilityOpenAICompatible      Compatibility = "openai_compatible"
	CompatibilityLegacyOpenAI          Compatibility = "openai"
)

const (
	CredentialKeyAPICompatibilitySnake = "api_compatibility"
	CredentialKeyAPICompatibilityCamel = "apiCompatibility"
	CredentialKeyBaseURLSnake          = "base_url"
	CredentialKeyBaseURLCamel          = "baseUrl"
	CredentialKeyHeaders               = "headers"
)

const (
	DefaultClientTimeout  = 10 * time.Minute
	DefaultRequestTimeout = 60 * time.Second
)

const (
	ChatRoleAssistant = "assistant"
	ChatRoleFunction  = "function"
	ChatRoleSystem    = "system"
	ChatRoleTool      = "tool"
	ChatRoleUser      = "user"
)
