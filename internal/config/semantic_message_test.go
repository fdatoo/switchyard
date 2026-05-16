package config

import (
	"errors"
	"testing"
)

func TestNormalizeApplyMessage_Default(t *testing.T) {
	got, err := NormalizeApplyMessage("")
	if err != nil {
		t.Fatalf("NormalizeApplyMessage: %v", err)
	}
	if got != DefaultApplyMessage {
		t.Fatalf("message = %q, want %q", got, DefaultApplyMessage)
	}
}

func TestNormalizeApplyMessage_Valid(t *testing.T) {
	got, err := NormalizeApplyMessage("  config(scene): add evening kitchen scene  ")
	if err != nil {
		t.Fatalf("NormalizeApplyMessage: %v", err)
	}
	if got != "config(scene): add evening kitchen scene" {
		t.Fatalf("message = %q", got)
	}
}

func TestNormalizeApplyMessage_Invalid(t *testing.T) {
	tests := []string{
		"cli golden",
		"feat(config): add config",
		"config(bogus): add config",
		"config(scene): Add scene",
		"config(scene): add scene.",
		"config(scene): add scene\nmore",
	}
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			_, err := NormalizeApplyMessage(tc)
			if !errors.Is(err, ErrInvalidSemanticMessage) {
				t.Fatalf("err = %v, want ErrInvalidSemanticMessage", err)
			}
		})
	}
}
