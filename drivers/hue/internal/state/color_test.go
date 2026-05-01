package state

import "testing"

func TestParseColor_Hex(t *testing.T) {
	cases := []struct {
		args                map[string]string
		wantR, wantG, wantB uint8
		wantErr             bool
	}{
		{map[string]string{"hex": "#FF8800"}, 0xFF, 0x88, 0x00, false},
		{map[string]string{"hex": "ff8800"}, 0xFF, 0x88, 0x00, false},
		{map[string]string{"hex": "#000000"}, 0, 0, 0, false},
		{map[string]string{"hex": "#GGGGGG"}, 0, 0, 0, true},
		{map[string]string{"hex": "#F80"}, 0, 0, 0, true},
		{map[string]string{"hex": "#FF880000"}, 0, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.args["hex"], func(t *testing.T) {
			r, g, b, err := ParseColor(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && (r != tc.wantR || g != tc.wantG || b != tc.wantB) {
				t.Errorf("(%d, %d, %d), want (%d, %d, %d)", r, g, b, tc.wantR, tc.wantG, tc.wantB)
			}
		})
	}
}

func TestParseColor_RGB(t *testing.T) {
	cases := []struct {
		name                string
		args                map[string]string
		wantR, wantG, wantB uint8
		wantErr             bool
	}{
		{"all three", map[string]string{"r": "255", "g": "136", "b": "0"}, 255, 136, 0, false},
		{"missing g", map[string]string{"r": "255", "b": "0"}, 0, 0, 0, true},
		{"r out of range", map[string]string{"r": "300", "g": "0", "b": "0"}, 0, 0, 0, true},
		{"non-numeric", map[string]string{"r": "x", "g": "0", "b": "0"}, 0, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, g, b, err := ParseColor(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && (r != tc.wantR || g != tc.wantG || b != tc.wantB) {
				t.Errorf("(%d, %d, %d), want (%d, %d, %d)", r, g, b, tc.wantR, tc.wantG, tc.wantB)
			}
		})
	}
}

func TestParseColor_HexBeatsRGB(t *testing.T) {
	args := map[string]string{
		"hex": "#FF0000",
		"r":   "0", "g": "255", "b": "0",
	}
	r, g, b, err := ParseColor(args)
	if err != nil {
		t.Fatal(err)
	}
	if r != 0xFF || g != 0 || b != 0 {
		t.Errorf("(%d, %d, %d), want (255, 0, 0) — hex should win", r, g, b)
	}
}

func TestParseColor_NeitherForm(t *testing.T) {
	if _, _, _, err := ParseColor(map[string]string{}); err == nil {
		t.Error("expected error when neither hex nor r/g/b is present")
	}
}
