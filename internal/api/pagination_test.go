package api_test

import (
	"testing"

	"github.com/fdatoo/switchyard/internal/api"
)

func TestCursor_Roundtrip(t *testing.T) {
	token, err := api.EncodeCursor(api.Cursor{Position: 4242, Tiebreak: "x"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := api.DecodeCursor(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Position != 4242 || got.Tiebreak != "x" {
		t.Errorf("got %+v", got)
	}
}

func TestCursor_EmptyIsZero(t *testing.T) {
	got, err := api.DecodeCursor("")
	if err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if got.Position != 0 || got.Tiebreak != "" {
		t.Errorf("got %+v, want zero", got)
	}
}

func TestCursor_Garbage(t *testing.T) {
	_, err := api.DecodeCursor("not-base64!!!")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClampPageSize(t *testing.T) {
	for _, tc := range []struct {
		in, out uint32
	}{
		{0, 100},
		{50, 50},
		{1000, 1000},
		{5000, 1000},
	} {
		if got := api.ClampPageSize(tc.in); got != tc.out {
			t.Errorf("ClampPageSize(%d) = %d, want %d", tc.in, got, tc.out)
		}
	}
}
