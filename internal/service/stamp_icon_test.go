package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestSanitizeStampIconAcceptsTransparentSquarePNG(t *testing.T) {
	data := testStampPNG(t, 64, 64, true)
	sanitized, err := sanitizeStampIcon(data)
	if err != nil {
		t.Fatalf("sanitize icon: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(sanitized))
	if err != nil {
		t.Fatalf("decode sanitized icon: %v", err)
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("unexpected sanitized dimensions: %v", img.Bounds())
	}
}

func TestSanitizeStampIconRejectsUnsafeImages(t *testing.T) {
	for name, data := range map[string][]byte{
		"opaque":     testStampPNG(t, 64, 64, false),
		"non-square": testStampPNG(t, 64, 65, true),
		"too-small":  testStampPNG(t, 32, 32, true),
		"invalid":    []byte("not a PNG"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := sanitizeStampIcon(data); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidatePassDesignRequestValidatesStampIconKey(t *testing.T) {
	for _, key := range []string{"none", "logo", "coffee", "star", "gift", "heart", "shopping_bag", "custom"} {
		input := validPassDesignRequest()
		input.StampIconKey = key
		if err := validatePassDesignRequest(input); err != nil {
			t.Fatalf("expected %q to validate: %v", key, err)
		}
	}
	input := validPassDesignRequest()
	input.StampIconKey = "diamond"
	if err := validatePassDesignRequest(input); err == nil {
		t.Fatal("expected unknown stamp icon key to fail")
	}
}

func TestNormalizePassDesignRequestDefaultsNewDesignToLogo(t *testing.T) {
	input := validPassDesignRequest()
	input.StampIconKey = ""
	normalizePassDesignRequest(&input)
	if input.StampIconKey != "logo" {
		t.Fatalf("expected new pass design to default to logo, got %q", input.StampIconKey)
	}
}

func testStampPNG(t *testing.T, width, height int, transparent bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	alpha := uint8(255)
	if transparent {
		alpha = 0
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 84, G: 52, B: 245, A: alpha})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
