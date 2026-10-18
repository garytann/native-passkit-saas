package applepass

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"

	xdraw "golang.org/x/image/draw"
)

const (
	MinArtworkSourceSize = 512
	MaxArtworkSourceSize = 4096
)

type artworkSize struct {
	width  int
	height int
}

var iconArtworkSizes = map[string]artworkSize{
	"icon.png":    {width: 38, height: 38},
	"icon@2x.png": {width: 76, height: 76},
	"icon@3x.png": {width: 114, height: 114},
}

var logoArtworkSizes = map[string]artworkSize{
	"logo.png":    {width: 160, height: 50},
	"logo@2x.png": {width: 320, height: 100},
	"logo@3x.png": {width: 480, height: 150},
}

// GenerateStandardArtwork derives every Apple Wallet scale variant directly
// from the uploaded source so no generated image is resized a second time.
func GenerateStandardArtwork(iconPNG []byte, logoPNG []byte) (map[string][]byte, error) {
	icon, sanitizedIcon, err := decodeArtworkSource("icon_source", iconPNG)
	if err != nil {
		return nil, err
	}
	logo, sanitizedLogo, err := decodeArtworkSource("logo_source", logoPNG)
	if err != nil {
		return nil, err
	}
	logoBounds, ok := visibleArtworkBounds(logo)
	if !ok {
		return nil, errors.New("logo_source must contain visible artwork")
	}

	assets := make(map[string][]byte, len(iconArtworkSizes)+len(logoArtworkSizes)+2)
	assets["source-icon.png"] = sanitizedIcon
	assets["source-logo.png"] = sanitizedLogo
	for name, size := range iconArtworkSizes {
		assets[name], err = renderContainedPNG(icon, size.width, size.height)
		if err != nil {
			return nil, fmt.Errorf("generate %s: %w", name, err)
		}
	}
	for name, size := range logoArtworkSizes {
		assets[name], err = renderTopLeftPNG(logo, logoBounds, size.width, size.height)
		if err != nil {
			return nil, fmt.Errorf("generate %s: %w", name, err)
		}
	}
	return assets, nil
}

func decodeArtworkSource(label string, data []byte) (image.Image, []byte, error) {
	if len(data) == 0 {
		return nil, nil, fmt.Errorf("%s is required", label)
	}
	configuration, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("%s must be a valid PNG", label)
	}
	if configuration.Width != configuration.Height || configuration.Width < MinArtworkSourceSize || configuration.Width > MaxArtworkSourceSize {
		return nil, nil, fmt.Errorf("%s must be square and between %d and %d pixels", label, MinArtworkSourceSize, MaxArtworkSourceSize)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("%s must be a valid PNG", label)
	}
	sanitized, err := encodeArtworkPNG(decoded)
	if err != nil {
		return nil, nil, fmt.Errorf("sanitize %s: %w", label, err)
	}
	return decoded, sanitized, nil
}

func renderContainedPNG(source image.Image, width int, height int) ([]byte, error) {
	if source == nil {
		return nil, errors.New("invalid artwork render dimensions")
	}
	return renderArtworkPNG(source, source.Bounds(), width, height, true)
}

func renderTopLeftPNG(source image.Image, sourceBounds image.Rectangle, width int, height int) ([]byte, error) {
	return renderArtworkPNG(source, sourceBounds, width, height, false)
}

func renderArtworkPNG(source image.Image, sourceBounds image.Rectangle, width int, height int, centered bool) ([]byte, error) {
	if source == nil || width <= 0 || height <= 0 {
		return nil, errors.New("invalid artwork render dimensions")
	}
	if sourceBounds.Empty() || !sourceBounds.In(source.Bounds()) {
		return nil, errors.New("invalid artwork source bounds")
	}
	scale := math.Min(float64(width)/float64(sourceBounds.Dx()), float64(height)/float64(sourceBounds.Dy()))
	renderWidth := min(width, max(1, int(math.Round(float64(sourceBounds.Dx())*scale))))
	renderHeight := min(height, max(1, int(math.Round(float64(sourceBounds.Dy())*scale))))
	offsetX, offsetY := 0, 0
	if centered {
		offsetX = (width - renderWidth) / 2
		offsetY = (height - renderHeight) / 2
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(
		canvas,
		image.Rect(offsetX, offsetY, offsetX+renderWidth, offsetY+renderHeight),
		source,
		sourceBounds,
		xdraw.Over,
		nil,
	)
	return encodeArtworkPNG(canvas)
}

func visibleArtworkBounds(source image.Image) (image.Rectangle, bool) {
	if source == nil || source.Bounds().Empty() {
		return image.Rectangle{}, false
	}

	bounds := source.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := source.At(x, y).RGBA()
			if alpha == 0 {
				continue
			}
			found = true
			minX = min(minX, x)
			minY = min(minY, y)
			maxX = max(maxX, x)
			maxY = max(maxY, y)
		}
	}
	if !found {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}

func encodeArtworkPNG(source image.Image) ([]byte, error) {
	var output bytes.Buffer
	if err := png.Encode(&output, source); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
