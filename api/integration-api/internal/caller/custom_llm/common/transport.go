package internal_custom_llm_common

import (
	"net/http"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func NewHTTPClient(config ClientConfig) *http.Client {
	return &http.Client{
		Timeout: config.ClientTimeout,
	}
}

func NewOpenAIClient(config ClientConfig) openai.Client {
	opts := []option.RequestOption{
		option.WithHTTPClient(NewHTTPClient(config)),
		option.WithRequestTimeout(config.RequestTimeout),
		option.WithBaseURL(config.BaseURL),
	}
	for key, value := range config.Headers {
		opts = append(opts, option.WithHeader(key, value))
	}
	return openai.NewClient(opts...)
}

func ApplyHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		if key == "" {
			continue
		}
		req.Header.Set(key, value)
	}
}
