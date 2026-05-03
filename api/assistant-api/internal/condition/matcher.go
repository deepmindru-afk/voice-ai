// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package condition

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Entry struct {
	Key       string `json:"key"`
	Condition string `json:"condition"`
	Value     string `json:"value"`
}

type Context struct {
	Source string
	Mode   string
	// Direction is expected to be inbound/outbound.
	Direction string
}

type Matcher struct{}

var allowedSources = map[string]struct{}{
	"all":        {},
	"sdk":        {},
	"web_plugin": {},
	"debugger":   {},
	"phone":      {},
}

var allowedModes = map[string]struct{}{
	"all":   {},
	"text":  {},
	"voice": {},
}

var allowedDirections = map[string]struct{}{
	"both":     {},
	"inbound":  {},
	"outbound": {},
}

func NewMatcher() *Matcher {
	return &Matcher{}
}

func (m *Matcher) Evaluate(raw string, ctx Context) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true, nil
	}

	var entries []Entry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return false, fmt.Errorf("failed to parse condition JSON: %w", err)
	}
	if len(entries) == 0 {
		return false, fmt.Errorf("condition must include at least one entry")
	}

	source := normalizeSource(ctx.Source)
	mode := normalizeMode(ctx.Mode)
	direction := normalizeDirection(ctx.Direction)

	for _, entry := range entries {
		key := normalizeKey(entry.Key)
		op := strings.TrimSpace(entry.Condition)
		if op != "=" {
			return false, fmt.Errorf("unsupported condition operator for %s: %s", key, op)
		}

		switch key {
		case "source":
			expected := normalizeSource(entry.Value)
			if _, ok := allowedSources[expected]; !ok {
				return false, fmt.Errorf("unsupported source condition value: %s", entry.Value)
			}
			if expected != "all" && expected != source {
				return false, nil
			}
		case "conversation_mode":
			expected := normalizeMode(entry.Value)
			if _, ok := allowedModes[expected]; !ok {
				return false, fmt.Errorf("unsupported conversation_mode condition value: %s", entry.Value)
			}
			if expected != "all" && expected != mode {
				return false, nil
			}
		case "direction":
			expected := normalizeDirection(entry.Value)
			if _, ok := allowedDirections[expected]; !ok {
				return false, fmt.Errorf("unsupported direction condition value: %s", entry.Value)
			}
			if expected != "both" && expected != direction {
				return false, nil
			}
		default:
			return false, fmt.Errorf("unsupported condition key: %s", entry.Key)
		}
	}

	return true, nil
}

func normalizeKey(raw string) string {
	key := strings.TrimSpace(strings.ToLower(raw))
	key = strings.ReplaceAll(key, "-", "_")
	if key == "mode" {
		return "conversation_mode"
	}
	return key
}

func normalizeSource(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	v = strings.ReplaceAll(v, "-", "_")
	switch v {
	case "webplugin":
		return "web_plugin"
	case "phonecall", "phone_call":
		return "phone"
	default:
		return v
	}
}

func normalizeMode(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	v = strings.ReplaceAll(v, "-", "_")
	switch v {
	case "audio":
		return "voice"
	default:
		return v
	}
}

func normalizeDirection(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	v = strings.ReplaceAll(v, "-", "_")
	switch v {
	case "all":
		return "both"
	default:
		return v
	}
}
