package applepass

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"sort"

	xdraw "golang.org/x/image/draw"
)

const (
	stampStripWidth  = 375
	stampStripHeight = 144
	maxVisualStamps  = 10
)

type StampStripOptions struct {
	IconKey         string
	ArtworkPath     string
	StampCount      int
	StampGoal       int
	BackgroundColor string
	ForegroundColor string
	LabelColor      string
}

func RenderStampStrips(options StampStripOptions) (map[string][]byte, error) {
	if options.IconKey == "none" || options.StampGoal < 1 || options.StampGoal > maxVisualStamps {
		return nil, nil
	}
	var artwork image.Image
	if options.IconKey == "logo" || options.IconKey == "custom" {
		file, err := os.Open(options.ArtworkPath)
		if err != nil {
			return nil, nil
		}
		defer file.Close()
		artwork, err = png.Decode(file)
		if err != nil {
			return nil, nil
		}
	} else if !isBuiltInStampIcon(options.IconKey) {
		return nil, nil
	}

	assets := make(map[string][]byte, 3)
	for scale, name := range map[int]string{1: "strip.png", 2: "strip@2x.png", 3: "strip@3x.png"} {
		img := renderStampStrip(options, artwork, scale)
		var output bytes.Buffer
		if err := png.Encode(&output, img); err != nil {
			return nil, fmt.Errorf("encode %s: %w", name, err)
		}
		assets[name] = output.Bytes()
	}
	return assets, nil
}

func renderStampStrip(options StampStripOptions, artwork image.Image, scale int) *image.NRGBA {
	width := stampStripWidth * scale
	height := stampStripHeight * scale
	background := parseStripColor(options.BackgroundColor, color.NRGBA{R: 84, G: 52, B: 245, A: 255})
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)

	goal := options.StampGoal
	count := minInt(maxInt(options.StampCount, 0), goal)
	margin := 22 * scale
	gap := 8 * scale
	available := width - 2*margin - (goal-1)*gap
	slotSize := minInt(42*scale, available/goal)
	rowWidth := goal*slotSize + (goal-1)*gap
	startX := (width - rowWidth) / 2
	startY := height - slotSize - 10*scale
	mask := builtInStampMask(options.IconKey)

	for index := 0; index < goal; index++ {
		rect := image.Rect(startX+index*(slotSize+gap), startY, startX+index*(slotSize+gap)+slotSize, startY+slotSize)
		drawPhysicalStampSpot(canvas, rect, artwork, mask, index < count, background)
	}
	return canvas
}

func drawPhysicalStampSpot(destination *image.NRGBA, rect image.Rectangle, artwork image.Image, mask *image.Alpha, filled bool, ink color.NRGBA) {
	drawCircle(destination, rect, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	if !filled {
		return
	}

	iconSize := maxInt(1, int(math.Round(float64(rect.Dx())*.65)))
	if artwork != nil {
		target := containedImageRect(rect, artwork.Bounds(), iconSize)
		scaled := image.NewNRGBA(image.Rect(0, 0, target.Dx(), target.Dy()))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), artwork, artwork.Bounds(), draw.Over, nil)
		draw.Draw(destination, target, scaled, scaled.Bounds().Min, draw.Over)
		return
	}

	iconRect := centeredRect(rect, iconSize, iconSize)
	scaledMask := image.NewAlpha(image.Rect(0, 0, iconSize, iconSize))
	xdraw.CatmullRom.Scale(scaledMask, scaledMask.Bounds(), mask, mask.Bounds(), draw.Src, nil)
	draw.DrawMask(destination, iconRect, &image.Uniform{C: ink}, image.Point{}, scaledMask, image.Point{}, draw.Over)
}

func containedImageRect(container image.Rectangle, source image.Rectangle, maximum int) image.Rectangle {
	width, height := maximum, maximum
	if source.Dx() > source.Dy() {
		height = maxInt(1, int(math.Round(float64(maximum)*float64(source.Dy())/float64(source.Dx()))))
	} else if source.Dy() > source.Dx() {
		width = maxInt(1, int(math.Round(float64(maximum)*float64(source.Dx())/float64(source.Dy()))))
	}
	return centeredRect(container, width, height)
}

func centeredRect(container image.Rectangle, width int, height int) image.Rectangle {
	x := container.Min.X + (container.Dx()-width)/2
	y := container.Min.Y + (container.Dy()-height)/2
	return image.Rect(x, y, x+width, y+height)
}

func drawCircle(destination draw.Image, rect image.Rectangle, fill color.Color) {
	cx := float64(rect.Min.X+rect.Max.X) / 2
	cy := float64(rect.Min.Y+rect.Max.Y) / 2
	radius := float64(minInt(rect.Dx(), rect.Dy())) / 2
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dx := float64(x) + .5 - cx
			dy := float64(y) + .5 - cy
			if dx*dx+dy*dy <= radius*radius {
				destination.Set(x, y, fill)
			}
		}
	}
}

func isBuiltInStampIcon(key string) bool {
	switch key {
	case "coffee", "star", "gift", "heart", "shopping_bag":
		return true
	default:
		return false
	}
}

func builtInStampMask(key string) *image.Alpha {
	const size = 256
	mask := image.NewAlpha(image.Rect(0, 0, size, size))
	switch key {
	case "star":
		points := make([]image.Point, 0, 10)
		for index := 0; index < 10; index++ {
			radius := 104.0
			if index%2 == 1 {
				radius = 45
			}
			angle := -math.Pi/2 + float64(index)*math.Pi/5
			points = append(points, image.Pt(128+int(math.Cos(angle)*radius), 128+int(math.Sin(angle)*radius)))
		}
		fillPolygon(mask, points, 255)
	case "heart":
		fillCircleAlpha(mask, image.Pt(86, 92), 55, 255)
		fillCircleAlpha(mask, image.Pt(170, 92), 55, 255)
		fillPolygon(mask, []image.Point{{38, 104}, {218, 104}, {128, 226}}, 255)
	case "gift":
		fillRectAlpha(mask, image.Rect(38, 92, 218, 222), 255)
		fillRectAlpha(mask, image.Rect(26, 70, 230, 108), 255)
		fillRectAlpha(mask, image.Rect(112, 70, 144, 222), 0)
		fillCircleAlpha(mask, image.Pt(94, 54), 35, 255)
		fillCircleAlpha(mask, image.Pt(162, 54), 35, 255)
		fillCircleAlpha(mask, image.Pt(94, 54), 17, 0)
		fillCircleAlpha(mask, image.Pt(162, 54), 17, 0)
		fillRectAlpha(mask, image.Rect(117, 38, 139, 92), 255)
	case "shopping_bag":
		fillPolygon(mask, []image.Point{{48, 76}, {208, 76}, {222, 224}, {34, 224}}, 255)
		fillCircleAlpha(mask, image.Pt(128, 78), 58, 255)
		fillCircleAlpha(mask, image.Pt(128, 78), 34, 0)
		fillRectAlpha(mask, image.Rect(56, 76, 200, 126), 0)
	default:
		fillRectAlpha(mask, image.Rect(46, 88, 184, 202), 255)
		fillCircleAlpha(mask, image.Pt(184, 139), 54, 255)
		fillCircleAlpha(mask, image.Pt(184, 139), 29, 0)
		fillRectAlpha(mask, image.Rect(166, 88, 210, 190), 0)
		fillRectAlpha(mask, image.Rect(34, 198, 206, 218), 255)
		fillRectAlpha(mask, image.Rect(78, 42, 92, 77), 255)
		fillRectAlpha(mask, image.Rect(121, 28, 135, 77), 255)
		fillRectAlpha(mask, image.Rect(164, 42, 178, 77), 255)
	}
	return mask
}

func fillRectAlpha(img *image.Alpha, rect image.Rectangle, alpha uint8) {
	draw.Draw(img, rect.Intersect(img.Bounds()), &image.Uniform{C: color.Alpha{A: alpha}}, image.Point{}, draw.Src)
}

func fillCircleAlpha(img *image.Alpha, center image.Point, radius int, alpha uint8) {
	rect := image.Rect(center.X-radius, center.Y-radius, center.X+radius+1, center.Y+radius+1).Intersect(img.Bounds())
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dx := x - center.X
			dy := y - center.Y
			if dx*dx+dy*dy <= radius*radius {
				img.SetAlpha(x, y, color.Alpha{A: alpha})
			}
		}
	}
}

func fillPolygon(img *image.Alpha, points []image.Point, alpha uint8) {
	if len(points) < 3 {
		return
	}
	minY, maxY := points[0].Y, points[0].Y
	for _, point := range points[1:] {
		minY = minInt(minY, point.Y)
		maxY = maxInt(maxY, point.Y)
	}
	for y := maxInt(minY, img.Bounds().Min.Y); y <= minInt(maxY, img.Bounds().Max.Y-1); y++ {
		var intersections []int
		for index, point := range points {
			next := points[(index+1)%len(points)]
			if (point.Y <= y && next.Y > y) || (next.Y <= y && point.Y > y) {
				x := point.X + (y-point.Y)*(next.X-point.X)/(next.Y-point.Y)
				intersections = append(intersections, x)
			}
		}
		sort.Ints(intersections)
		for i := 0; i+1 < len(intersections); i += 2 {
			start, end := intersections[i], intersections[i+1]
			if start > end {
				start, end = end, start
			}
			fillRectAlpha(img, image.Rect(start, y, end+1, y+1), alpha)
		}
	}
}

func parseStripColor(value string, fallback color.NRGBA) color.NRGBA {
	if len(value) != 7 || value[0] != '#' {
		return fallback
	}
	var red, green, blue uint8
	if _, err := fmt.Sscanf(value, "#%02x%02x%02x", &red, &green, &blue); err != nil {
		return fallback
	}
	return color.NRGBA{R: red, G: green, B: blue, A: 255}
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
