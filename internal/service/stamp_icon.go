package service

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"strings"
)

const maxStampIconUploadBytes = 1 << 20

var stampIconKeys = map[string]struct{}{
	"none":         {},
	"logo":         {},
	"coffee":       {},
	"star":         {},
	"gift":         {},
	"heart":        {},
	"shopping_bag": {},
	"custom":       {},
}

func validStampIconKey(value string) bool {
	_, ok := stampIconKeys[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

func sanitizeStampIcon(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ValidationError{Message: "stamp_icon is required when stamp_icon_key is custom"}
	}
	if len(data) > maxStampIconUploadBytes {
		return nil, ValidationError{Message: "stamp_icon exceeds the 1 MB upload limit"}
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ValidationError{Message: "stamp_icon must be a valid PNG"}
	}
	bounds := img.Bounds()
	if bounds.Dx() != bounds.Dy() || bounds.Dx() < 64 || bounds.Dx() > 512 {
		return nil, ValidationError{Message: "stamp_icon must be square and between 64 and 512 pixels"}
	}
	if !imageHasTransparency(img) {
		return nil, ValidationError{Message: "stamp_icon must have a transparent background"}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		return nil, fmt.Errorf("sanitize stamp icon: %w", err)
	}
	return output.Bytes(), nil
}

func imageHasTransparency(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha < 0xffff {
				return true
			}
		}
	}
	return false
}
