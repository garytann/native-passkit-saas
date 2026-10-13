package googlewallet

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const (
	HeroWidth  = 1032
	HeroHeight = 812
)

type StampImageOptions struct {
	ArtworkPath     string
	StampCount      int
	StampGoal       int
	BackgroundColor string
}

func RenderStampImage(options StampImageOptions) ([]byte, error) {
	if options.StampGoal < 1 || options.StampGoal > 10 || strings.TrimSpace(options.ArtworkPath) == "" {
		return nil, nil
	}
	artworkFile, err := os.Open(options.ArtworkPath)
	if err != nil {
		return nil, nil
	}
	defer artworkFile.Close()
	artwork, _, err := image.Decode(artworkFile)
	if err != nil {
		return nil, fmt.Errorf("decode Google stamp artwork: %w", err)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, HeroWidth, HeroHeight))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: parseHexColor(options.BackgroundColor)}, image.Point{}, draw.Src)
	rows := [][]int{}
	if options.StampGoal <= 5 {
		rows = append(rows, makeIndexes(0, options.StampGoal))
	} else {
		top := (options.StampGoal + 1) / 2
		rows = append(rows, makeIndexes(0, top), makeIndexes(top, options.StampGoal))
	}
	diameter := 150
	if options.StampGoal <= 4 {
		diameter = 176
	}
	rowGap := 74
	totalHeight := len(rows)*diameter + (len(rows)-1)*rowGap
	startY := (HeroHeight - totalHeight) / 2
	for rowIndex, row := range rows {
		gap := 34
		totalWidth := len(row)*diameter + (len(row)-1)*gap
		startX := (HeroWidth - totalWidth) / 2
		for column, position := range row {
			x := startX + column*(diameter+gap)
			y := startY + rowIndex*(diameter+rowGap)
			drawCircle(canvas, x+diameter/2, y+diameter/2, diameter/2, color.White)
			if position < options.StampCount {
				drawArtwork(canvas, artwork, image.Rect(x, y, x+diameter, y+diameter))
			}
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, fmt.Errorf("encode Google stamp artwork: %w", err)
	}
	return output.Bytes(), nil
}

func makeIndexes(start, end int) []int {
	values := make([]int, 0, end-start)
	for value := start; value < end; value++ {
		values = append(values, value)
	}
	return values
}

func drawCircle(destination *image.RGBA, centerX, centerY, radius int, fill color.Color) {
	radiusSquared := radius * radius
	for y := centerY - radius; y <= centerY+radius; y++ {
		for x := centerX - radius; x <= centerX+radius; x++ {
			dx, dy := x-centerX, y-centerY
			if dx*dx+dy*dy <= radiusSquared {
				destination.Set(x, y, fill)
			}
		}
	}
}

func drawArtwork(destination *image.RGBA, artwork image.Image, slot image.Rectangle) {
	bounds := artwork.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 {
		return
	}
	maxSize := int(float64(slot.Dx()) * 0.68)
	width, height := maxSize, maxSize
	if bounds.Dx() > bounds.Dy() {
		height = maxSize * bounds.Dy() / bounds.Dx()
	} else {
		width = maxSize * bounds.Dx() / bounds.Dy()
	}
	x := slot.Min.X + (slot.Dx()-width)/2
	y := slot.Min.Y + (slot.Dy()-height)/2
	xdraw.CatmullRom.Scale(destination, image.Rect(x, y, x+width, y+height), artwork, bounds, draw.Over, nil)
}

func parseHexColor(value string) color.RGBA {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	var red, green, blue uint8
	if len(value) == 6 {
		_, _ = fmt.Sscanf(value, "%02x%02x%02x", &red, &green, &blue)
	}
	if red == 0 && green == 0 && blue == 0 && strings.ToLower(value) != "000000" {
		return color.RGBA{R: 84, G: 52, B: 245, A: 255}
	}
	return color.RGBA{R: red, G: green, B: blue, A: 255}
}
