package web

import (
	"bytes"
	"image/png"
	"testing"
)

func TestPixelFontGlyphs(t *testing.T) {
	for r, g := range pixelFont {
		if len(g) != 35 {
			t.Errorf("%q: %d pixels, want 35", r, len(g))
		}
	}
}

func TestPixelUpper(t *testing.T) {
	for in, want := range map[string]string{"Antoine": "ANTOINE", "Hélène  Ünal": "HELENE UNAL", "ß_42🎉": "SS 42"} {
		if got := pixelUpper(in); got != want {
			t.Errorf("pixelUpper(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGameCardIsPNG(t *testing.T) {
	score := 62
	for _, s := range []*int{nil, &score} {
		img, err := png.Decode(bytes.NewReader(gameCard("Antoine", s)))
		if err != nil || img.Bounds().Dx() != cardW || img.Bounds().Dy() != cardH {
			t.Fatalf("card: %v %v", err, img)
		}
	}
}
