package applepass

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestGenerateStandardArtworkProducesExpectedSizes(t *testing.T) {
	source := artworkTestPNG(t, 512, 512, color.NRGBA{R: 24, G: 112, B: 205, A: 255})
	assets, err := GenerateStandardArtwork(source, source)
	if err != nil {
		t.Fatalf("generate artwork: %v", err)
	}

	expected := map[string]image.Point{
		"icon.png":        {X: 38, Y: 38},
		"icon@2x.png":     {X: 76, Y: 76},
		"icon@3x.png":     {X: 114, Y: 114},
		"logo.png":        {X: 120, Y: 50},
		"logo@2x.png":     {X: 240, Y: 100},
		"logo@3x.png":     {X: 360, Y: 150},
		"source-icon.png": {X: 512, Y: 512},
		"source-logo.png": {X: 512, Y: 512},
	}
	for name, size := range expected {
		configuration, err := png.DecodeConfig(bytes.NewReader(assets[name]))
		if err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if configuration.Width != size.X || configuration.Height != size.Y {
			t.Errorf("%s dimensions = %dx%d, want %dx%d", name, configuration.Width, configuration.Height, size.X, size.Y)
		}
	}
}

func TestGenerateStandardArtworkCentersLogoOnTransparentCanvas(t *testing.T) {
	source := artworkTestPNG(t, 512, 512, color.NRGBA{R: 220, G: 45, B: 80, A: 255})
	assets, err := GenerateStandardArtwork(source, source)
	if err != nil {
		t.Fatalf("generate artwork: %v", err)
	}

	logo, err := png.Decode(bytes.NewReader(assets["logo.png"]))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, alpha := logo.At(0, 25).RGBA(); alpha != 0 {
		t.Fatalf("expected transparent horizontal padding, alpha = %d", alpha)
	}
	red, green, blue, alpha := logo.At(60, 25).RGBA()
	if alpha != 0xffff || red < 0xd000 || green > 0x4000 || blue > 0x7000 {
		t.Fatalf("expected source color at canvas center, got rgba(%d,%d,%d,%d)", red, green, blue, alpha)
	}
	if _, _, _, alpha := logo.At(119, 25).RGBA(); alpha != 0 {
		t.Fatalf("expected transparent horizontal padding, alpha = %d", alpha)
	}
}

func TestGenerateStandardArtworkRejectsInvalidSources(t *testing.T) {
	valid := artworkTestPNG(t, 512, 512, color.NRGBA{A: 255})
	jpegSource := artworkTestJPEG(t, 512, 512)
	tests := []struct {
		name      string
		icon      []byte
		logo      []byte
		wantError string
	}{
		{name: "missing icon", icon: nil, logo: valid, wantError: "icon_source is required"},
		{name: "invalid png", icon: jpegSource, logo: valid, wantError: "icon_source must be a valid PNG"},
		{name: "not square", icon: artworkTestPNG(t, 512, 640, color.NRGBA{A: 255}), logo: valid, wantError: "icon_source must be square"},
		{name: "too small", icon: artworkTestPNG(t, 511, 511, color.NRGBA{A: 255}), logo: valid, wantError: "icon_source must be square"},
		{name: "too large", icon: valid, logo: artworkTestPNG(t, 4097, 4097, color.NRGBA{A: 255}), wantError: "logo_source must be square"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := GenerateStandardArtwork(test.icon, test.logo)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func artworkTestPNG(t *testing.T, width int, height int, fill color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func artworkTestJPEG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var output bytes.Buffer
	if err := jpeg.Encode(&output, img, nil); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
