// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package condition

import "testing"

func TestEvaluate_EmptyCondition_Allows(t *testing.T) {
	m := NewMatcher()
	ok, err := m.Evaluate("", Context{Source: "phone-call", Mode: "audio", Direction: "inbound"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !ok {
		t.Fatalf("expected empty condition to allow")
	}
}

func TestEvaluate_SourceModeDirection_Match(t *testing.T) {
	m := NewMatcher()
	raw := `[{"key":"source","condition":"=","value":"phone"},{"key":"conversation_mode","condition":"=","value":"voice"},{"key":"direction","condition":"=","value":"inbound"}]`
	ok, err := m.Evaluate(raw, Context{Source: "phone-call", Mode: "audio", Direction: "inbound"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !ok {
		t.Fatalf("expected condition to match")
	}
}

func TestEvaluate_SourceMismatch_Blocks(t *testing.T) {
	m := NewMatcher()
	ok, err := m.Evaluate(`[{"key":"source","condition":"=","value":"sdk"}]`, Context{Source: "phone-call"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ok {
		t.Fatalf("expected source mismatch to block")
	}
}

func TestEvaluate_ModeMismatch_Blocks(t *testing.T) {
	m := NewMatcher()
	ok, err := m.Evaluate(`[{"key":"conversation_mode","condition":"=","value":"text"}]`, Context{Mode: "audio"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ok {
		t.Fatalf("expected mode mismatch to block")
	}
}

func TestEvaluate_DirectionMismatch_Blocks(t *testing.T) {
	m := NewMatcher()
	ok, err := m.Evaluate(`[{"key":"direction","condition":"=","value":"outbound"}]`, Context{Direction: "inbound"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ok {
		t.Fatalf("expected direction mismatch to block")
	}
}

func TestEvaluate_AllAndBoth_Allow(t *testing.T) {
	m := NewMatcher()
	raw := `[{"key":"source","condition":"=","value":"all"},{"key":"conversation_mode","condition":"=","value":"all"},{"key":"direction","condition":"=","value":"both"}]`
	ok, err := m.Evaluate(raw, Context{Source: "sdk", Mode: "text", Direction: "outbound"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !ok {
		t.Fatalf("expected all/both to allow")
	}
}

func TestEvaluate_UnsupportedKey_ReturnsError(t *testing.T) {
	m := NewMatcher()
	_, err := m.Evaluate(`[{"key":"region","condition":"=","value":"sg"}]`, Context{})
	if err == nil {
		t.Fatalf("expected unsupported key error")
	}
}

func TestEvaluate_UnsupportedOperator_ReturnsError(t *testing.T) {
	m := NewMatcher()
	_, err := m.Evaluate(`[{"key":"source","condition":"!=","value":"phone"}]`, Context{})
	if err == nil {
		t.Fatalf("expected unsupported operator error")
	}
}
