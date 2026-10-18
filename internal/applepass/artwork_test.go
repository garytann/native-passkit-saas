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
		"logo.png":        {X: 160, Y: 50},
		"logo@2x.png":     {X: 320, Y: 100},
		"logo@3x.png":     {X: 480, Y: 150},
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

func TestGenerateStandardArtworkTrimsAndTopLeftAlignsLogo(t *testing.T) {
	icon := artworkTestPNG(t, 512, 512, color.NRGBA{R: 24, G: 112, B: 205, A: 255})
	logo := artworkTestPaddedPNG(t, 512, 512, image.Rect(120, 80, 440, 240), color.NRGBA{R: 220, G: 45, B: 80, A: 255})
	assets, err := GenerateStandardArtwork(icon, logo)
	if err != nil {
		t.Fatalf("generate artwork: %v", err)
	}

	generatedLogo, err := png.Decode(bytes.NewReader(assets["logo.png"]))
	if err != nil {
		t.Fatal(err)
	}
	red, green, blue, alpha := generatedLogo.At(0, 0).RGBA()
	if alpha != 0xffff || red < 0xd000 || green > 0x4000 || blue > 0x7000 {
		t.Fatalf("expected source color at top-left, got rgba(%d,%d,%d,%d)", red, green, blue, alpha)
	}
	red, green, blue, alpha = generatedLogo.At(99, 49).RGBA()
	if alpha != 0xffff || red < 0xd000 || green > 0x4000 || blue > 0x7000 {
		t.Fatalf("expected scaled source color at visible bottom-right, got rgba(%d,%d,%d,%d)", red, green, blue, alpha)
	}
	if _, _, _, alpha := generatedLogo.At(100, 25).RGBA(); alpha != 0 {
		t.Fatalf("expected remaining canvas to stay transparent, alpha = %d", alpha)
	}
}

func TestGenerateStandardArtworkTopLeftAlignsOpaqueLogo(t *testing.T) {
	source := artworkTestPNG(t, 512, 512, color.NRGBA{R: 28, G: 160, B: 90, A: 255})
	assets, err := GenerateStandardArtwork(source, source)
	if err != nil {
		t.Fatalf("generate artwork: %v", err)
	}

	logo, err := png.Decode(bytes.NewReader(assets["logo.png"]))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, alpha := logo.At(0, 0).RGBA(); alpha != 0xffff {
		t.Fatalf("expected opaque source at top-left, alpha = %d", alpha)
	}
	if _, _, _, alpha := logo.At(49, 49).RGBA(); alpha != 0xffff {
		t.Fatalf("expected opaque source at scaled bottom-right, alpha = %d", alpha)
	}
	if _, _, _, alpha := logo.At(50, 25).RGBA(); alpha != 0 {
		t.Fatalf("expected unused canvas to stay transparent, alpha = %d", alpha)
	}
}

func TestGenerateStandardArtworkPreservesTallLogoAspectRatio(t *testing.T) {
	icon := artworkTestPNG(t, 512, 512, color.NRGBA{A: 255})
	logoSource := artworkTestPaddedPNG(t, 512, 512, image.Rect(176, 96, 336, 416), color.NRGBA{B: 220, A: 255})
	assets, err := GenerateStandardArtwork(icon, logoSource)
	if err != nil {
		t.Fatalf("generate artwork: %v", err)
	}

	logo, err := png.Decode(bytes.NewReader(assets["logo.png"]))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, alpha := logo.At(24, 49).RGBA(); alpha != 0xffff {
		t.Fatalf("expected tall artwork to occupy 25x50 pixels, alpha = %d", alpha)
	}
	if _, _, _, alpha := logo.At(25, 25).RGBA(); alpha != 0 {
		t.Fatalf("expected aspect-ratio-preserving transparent space, alpha = %d", alpha)
	}
}

func TestVisibleArtworkBoundsPreservesPartiallyTransparentEdges(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	img.SetNRGBA(3, 4, color.NRGBA{R: 255, A: 1})
	img.SetNRGBA(14, 15, color.NRGBA{R: 255, A: 255})

	bounds, ok := visibleArtworkBounds(img)
	if !ok {
		t.Fatal("expected visible artwork bounds")
	}
	want := image.Rect(3, 4, 15, 16)
	if bounds != want {
		t.Fatalf("visible bounds = %v, want %v", bounds, want)
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
		{name: "transparent logo", icon: valid, logo: artworkTestPNG(t, 512, 512, color.NRGBA{}), wantError: "logo_source must contain visible artwork"},
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

func artworkTestPaddedPNG(t *testing.T, width int, height int, visible image.Rectangle, fill color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := visible.Min.Y; y < visible.Max.Y; y++ {
		for x := visible.Min.X; x < visible.Max.X; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
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
