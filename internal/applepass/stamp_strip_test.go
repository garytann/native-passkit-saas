package applepass

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderStampStripsCreatesWalletScaleAssets(t *testing.T) {
	assets, err := RenderStampStrips(StampStripOptions{
		IconKey:         "coffee",
		StampCount:      6,
		StampGoal:       10,
		BackgroundColor: "#5434F5",
		ForegroundColor: "#FFFFFF",
		LabelColor:      "#E1DCFF",
	})
	if err != nil {
		t.Fatalf("render stamp strips: %v", err)
	}

	for name, expected := range map[string][2]int{
		"strip.png":    {375, 144},
		"strip@2x.png": {750, 288},
		"strip@3x.png": {1125, 432},
	} {
		img, err := png.Decode(bytes.NewReader(assets[name]))
		if err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if img.Bounds().Dx() != expected[0] || img.Bounds().Dy() != expected[1] {
			t.Fatalf("unexpected %s dimensions: %v", name, img.Bounds())
		}
	}
}

func TestRenderStampStripsChangesWithProgress(t *testing.T) {
	options := StampStripOptions{IconKey: "star", StampGoal: 10, BackgroundColor: "#5434F5", ForegroundColor: "#FFFFFF", LabelColor: "#E1DCFF"}
	empty, err := RenderStampStrips(options)
	if err != nil {
		t.Fatal(err)
	}
	options.StampCount = 6
	partial, err := RenderStampStrips(options)
	if err != nil {
		t.Fatal(err)
	}
	options.StampCount = 10
	complete, err := RenderStampStrips(options)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(empty["strip.png"], partial["strip.png"]) || bytes.Equal(partial["strip.png"], complete["strip.png"]) {
		t.Fatal("expected stamp progress to produce different strip artwork")
	}
}

func TestRenderStampStripsDrawsExpectedFilledCount(t *testing.T) {
	for _, expected := range []int{0, 6, 10} {
		assets, err := RenderStampStrips(StampStripOptions{
			IconKey:         "star",
			StampCount:      expected,
			StampGoal:       10,
			BackgroundColor: "#5434F5",
			ForegroundColor: "#FFFFFF",
			LabelColor:      "#E1DCFF",
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := countFilledStampSlots(t, assets["strip.png"], 10); got != expected {
			t.Fatalf("expected %d filled stamp slots, got %d", expected, got)
		}
		assertWhitePhysicalSlots(t, assets["strip.png"], 10)
	}
}

func TestStampSlotRectsUseAdaptiveBalancedRows(t *testing.T) {
	for _, test := range []struct {
		goal     int
		rowSizes []int
	}{
		{goal: 1, rowSizes: []int{1}},
		{goal: 5, rowSizes: []int{5}},
		{goal: 6, rowSizes: []int{3, 3}},
		{goal: 7, rowSizes: []int{4, 3}},
		{goal: 9, rowSizes: []int{5, 4}},
		{goal: 10, rowSizes: []int{5, 5}},
	} {
		t.Run(fmt.Sprintf("goal_%d", test.goal), func(t *testing.T) {
			positions := stampSlotRects(test.goal, 1)
			if len(positions) != test.goal {
				t.Fatalf("expected %d positions, got %d", test.goal, len(positions))
			}
			rows := make(map[int]int)
			var rowOrder []int
			for _, position := range positions {
				if position.Dx() != stampSpotDiameter || position.Dy() != stampSpotDiameter {
					t.Fatalf("unexpected stamp dimensions: %v", position)
				}
				if _, exists := rows[position.Min.Y]; !exists {
					rowOrder = append(rowOrder, position.Min.Y)
				}
				rows[position.Min.Y]++
			}
			if len(rowOrder) != len(test.rowSizes) {
				t.Fatalf("expected %d rows, got %d", len(test.rowSizes), len(rowOrder))
			}
			for index, expected := range test.rowSizes {
				if got := rows[rowOrder[index]]; got != expected {
					t.Fatalf("row %d: expected %d stamps, got %d", index, expected, got)
				}
			}
		})
	}
}

func TestRenderStampStripsFillsTopRowBeforeBottomRow(t *testing.T) {
	assets, err := RenderStampStrips(StampStripOptions{
		IconKey:         "star",
		StampCount:      5,
		StampGoal:       7,
		BackgroundColor: "#5434F5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := countFilledStampSlots(t, assets["strip.png"], 7); got != 5 {
		t.Fatalf("expected five filled stamps across rows, got %d", got)
	}
}

func TestRenderStampStripsPreservesUploadedArtwork(t *testing.T) {
	for name, size := range map[string]image.Point{
		"square":      image.Pt(96, 96),
		"rectangular": image.Pt(128, 64),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "artwork.png")
			writeArtworkPNG(t, path, size.X, size.Y, true)
			assets, err := RenderStampStrips(StampStripOptions{IconKey: "logo", ArtworkPath: path, StampCount: 1, StampGoal: 3, BackgroundColor: "#5434F5"})
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(assets["strip.png"]))
			if err != nil {
				t.Fatal(err)
			}
			if !imageContainsColor(img, color.NRGBA{R: 220, G: 35, B: 70, A: 255}) || !imageContainsColor(img, color.NRGBA{R: 25, G: 140, B: 220, A: 255}) {
				t.Fatal("expected uploaded artwork colors to be preserved")
			}
			assertWhitePhysicalSlots(t, assets["strip.png"], 3)
		})
	}
}

func TestRenderStampStripsAcceptsOpaqueLogoArtwork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logo.png")
	writeArtworkPNG(t, path, 87, 87, false)
	assets, err := RenderStampStrips(StampStripOptions{IconKey: "logo", ArtworkPath: path, StampCount: 1, StampGoal: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 3 {
		t.Fatalf("expected three strip assets, got %d", len(assets))
	}
}

func TestRenderStampStripsFallsBackWithoutArtwork(t *testing.T) {
	for _, options := range []StampStripOptions{
		{IconKey: "none", StampGoal: 10},
		{IconKey: "coffee", StampGoal: 11},
		{IconKey: "logo", StampGoal: 10, ArtworkPath: "/missing/icon@3x.png"},
		{IconKey: "custom", StampGoal: 10, ArtworkPath: "/missing/stamp.png"},
	} {
		assets, err := RenderStampStrips(options)
		if err != nil {
			t.Fatalf("expected safe fallback, got %v", err)
		}
		if len(assets) != 0 {
			t.Fatalf("expected no strip assets for %+v", options)
		}
	}
}

func TestStampProgressChangesManifestHash(t *testing.T) {
	manifestHash := func(progress int) string {
		t.Helper()
		assets, err := RenderStampStrips(StampStripOptions{IconKey: "gift", StampCount: progress, StampGoal: 10})
		if err != nil {
			t.Fatal(err)
		}
		directory := t.TempDir()
		if err := writePassFiles(directory, Pass{FormatVersion: 1}, "", assets); err != nil {
			t.Fatal(err)
		}
		manifest, err := buildManifest(directory)
		if err != nil {
			t.Fatal(err)
		}
		return manifest["strip.png"]
	}

	if manifestHash(2) == manifestHash(7) {
		t.Fatal("expected per-member stamp progress to change the strip manifest hash")
	}
}

func countFilledStampSlots(t *testing.T, data []byte, goal int) int {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	filled := 0
	for _, position := range stampSlotRects(goal, 1) {
		centerX := position.Min.X + position.Dx()/2
		centerY := position.Min.Y + position.Dy()/2
		red, green, blue, _ := img.At(centerX, centerY).RGBA()
		if red < 0xf000 || green < 0xf000 || blue < 0xf000 {
			filled++
		}
	}
	return filled
}

func assertWhitePhysicalSlots(t *testing.T, data []byte, goal int) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for slot, position := range stampSlotRects(goal, 1) {
		x := position.Min.X + position.Dx()/2
		red, green, blue, _ := img.At(x, position.Min.Y+2).RGBA()
		if red < 0xf000 || green < 0xf000 || blue < 0xf000 {
			t.Fatalf("expected slot %d to have a white physical stamp circle", slot)
		}
	}
}

func writeArtworkPNG(t *testing.T, path string, width int, height int, transparent bool) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	backgroundAlpha := uint8(255)
	if transparent {
		backgroundAlpha = 0
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixel := color.NRGBA{R: 220, G: 35, B: 70, A: 255}
			if x >= width/2 {
				pixel = color.NRGBA{R: 25, G: 140, B: 220, A: 255}
			}
			if transparent && (x < width/8 || x >= width-width/8 || y < height/8 || y >= height-height/8) {
				pixel.A = backgroundAlpha
			}
			img.SetNRGBA(x, y, pixel)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func imageContainsColor(img image.Image, wanted color.NRGBA) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			red, green, blue, alpha := img.At(x, y).RGBA()
			if uint8(red>>8) == wanted.R && uint8(green>>8) == wanted.G && uint8(blue>>8) == wanted.B && uint8(alpha>>8) == wanted.A {
				return true
			}
		}
	}
	return false
}
