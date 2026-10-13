package googlewallet

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderStampImage(t *testing.T) {
	artworkPath := filepath.Join(t.TempDir(), "logo.png")
	logo := image.NewRGBA(image.Rect(0, 0, 80, 80))
	for y := 12; y < 68; y++ {
		for x := 12; x < 68; x++ {
			logo.Set(x, y, color.RGBA{R: 25, G: 135, B: 84, A: 255})
		}
	}
	writeTestPNG(t, artworkPath, logo)

	zero, err := RenderStampImage(StampImageOptions{ArtworkPath: artworkPath, StampGoal: 7, StampCount: 0, BackgroundColor: "#5434F5"})
	if err != nil {
		t.Fatalf("render empty progress: %v", err)
	}
	partial, err := RenderStampImage(StampImageOptions{ArtworkPath: artworkPath, StampGoal: 7, StampCount: 3, BackgroundColor: "#5434F5"})
	if err != nil {
		t.Fatalf("render partial progress: %v", err)
	}
	if bytes.Equal(zero, partial) {
		t.Fatal("expected progress to change the generated hero image")
	}

	decoded, err := png.Decode(bytes.NewReader(partial))
	if err != nil {
		t.Fatalf("decode generated image: %v", err)
	}
	if decoded.Bounds().Dx() != HeroWidth || decoded.Bounds().Dy() != HeroHeight {
		t.Fatalf("generated dimensions = %dx%d, want %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy(), HeroWidth, HeroHeight)
	}
	wantBackground := color.RGBA{R: 84, G: 52, B: 245, A: 255}
	if got := color.RGBAModel.Convert(decoded.At(0, 0)).(color.RGBA); got != wantBackground {
		t.Fatalf("background = %#v, want %#v", got, wantBackground)
	}
}

func TestRenderStampImageFallbacks(t *testing.T) {
	for _, test := range []StampImageOptions{
		{StampGoal: 0},
		{StampGoal: 11, ArtworkPath: "unused.png"},
		{StampGoal: 10, ArtworkPath: filepath.Join(t.TempDir(), "missing.png")},
	} {
		data, err := RenderStampImage(test)
		if err != nil {
			t.Fatalf("fallback returned error: %v", err)
		}
		if data != nil {
			t.Fatal("fallback should omit the hero image")
		}
	}
}

func writeTestPNG(t *testing.T, path string, source image.Image) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create artwork: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, source); err != nil {
		t.Fatalf("encode artwork: %v", err)
	}
}
