// Package render turns Telegram messages into a 4-gray bitmap for the
// Waveshare Pico-ePaper-3.7 display.
//
// Rendering happens on a landscape CanvasW x CanvasH image (natural for reading
// text). Pack then rotates and packs it into the panel's native 280x480
// GS2_HMSB framebuffer (see pack.go).
package render

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"time"

	"github.com/alexxrocco/telegram-epaper-display/backend/internal/store"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Landscape canvas dimensions (the panel's usable area, read in landscape).
const (
	CanvasW = 480
	CanvasH = 280
)

// Grayscale shades used while drawing. Quantization to the panel's 4 levels
// happens in pack.go, but using these values keeps the preview faithful.
var (
	white    = image.NewUniform(color.Gray{Y: 255})
	black    = image.NewUniform(color.Gray{Y: 0})
	darkGray = image.NewUniform(color.Gray{Y: 96})
)

// View is everything needed to render one frame.
type View struct {
	ChannelTitle string
	Messages     []store.Message // newest first
	UpdatedAt    time.Time
	// Empty is set when there are no messages yet, to show a placeholder.
	Empty bool
}

type faces struct {
	title font.Face
	meta  font.Face
	body  font.Face
}

var loadedFaces = mustLoadFaces()

func mustLoadFaces() faces {
	return faces{
		title: mustFace(gobold.TTF, 18),
		meta:  mustFace(goregular.TTF, 12),
		body:  mustFace(goregular.TTF, 16),
	}
}

func mustFace(ttf []byte, points float64) font.Face {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic("render: parse font: " + err.Error())
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    points,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		panic("render: new face: " + err.Error())
	}
	return face
}

// Render draws the view onto a landscape CanvasW x CanvasH grayscale image.
func Render(v View) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, CanvasW, CanvasH))
	draw.Draw(img, img.Bounds(), white, image.Point{}, draw.Src)

	headerH := drawHeader(img, v)
	y := headerH

	if v.Empty || len(v.Messages) == 0 {
		drawCentered(img, loadedFaces.body, "Waiting for messages…", headerH)
		return img
	}

	const (
		padX   = 8
		gap    = 6
		lineH  = 19 // body line height in px
		metaH  = 15
		sepGap = 4
	)
	maxW := CanvasW - 2*padX

	for _, m := range v.Messages {
		if y+metaH+lineH > CanvasH {
			break
		}
		// Meta line: time and optional author.
		meta := m.Date.Format("02 Jan 15:04")
		if m.Author != "" {
			meta += " · " + m.Author
		}
		y += metaH
		drawString(img, loadedFaces.meta, darkGray, padX, y-3, meta)

		// Body: wrapped text.
		lines := wrap(loadedFaces.body, m.Text, maxW)
		for _, ln := range lines {
			if y+lineH > CanvasH {
				break
			}
			y += lineH
			drawString(img, loadedFaces.body, black, padX, y-4, ln)
		}

		y += gap
		if y+1 < CanvasH {
			drawHLine(img, padX, CanvasW-padX, y, darkGray)
		}
		y += sepGap
	}
	return img
}

// drawHeader draws the top title bar and returns its height in px.
func drawHeader(img *image.Gray, v View) int {
	const h = 30
	draw.Draw(img, image.Rect(0, 0, CanvasW, h), black, image.Point{}, draw.Src)

	title := v.ChannelTitle
	if title == "" {
		title = "Telegram"
	}
	drawString(img, loadedFaces.title, white, 8, 21, title)

	stamp := v.UpdatedAt.Format("15:04")
	w := measure(loadedFaces.meta, stamp)
	drawString(img, loadedFaces.meta, white, CanvasW-8-w, 20, stamp)
	return h
}

func drawCentered(img *image.Gray, face font.Face, s string, topY int) {
	w := measure(face, s)
	x := (CanvasW - w) / 2
	y := topY + (CanvasH-topY)/2
	drawString(img, face, darkGray, x, y, s)
}

// drawString draws s with its baseline at (x, y).
func drawString(img *image.Gray, face font.Face, src image.Image, x, y int, s string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  src,
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

func drawHLine(img *image.Gray, x0, x1, y int, src image.Image) {
	draw.Draw(img, image.Rect(x0, y, x1, y+1), src, image.Point{}, draw.Src)
}

// measure returns the rendered width of s in px.
func measure(face font.Face, s string) int {
	return font.MeasureString(face, s).Round()
}

// wrap breaks text into lines no wider than maxW px, honoring existing newlines
// and breaking on spaces (falling back to character breaks for long words).
func wrap(face font.Face, text string, maxW int) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimRight(para, " \t\r")
		if para == "" {
			out = append(out, "")
			continue
		}
		words := strings.Fields(para)
		line := ""
		for _, word := range words {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if measure(face, candidate) <= maxW {
				line = candidate
				continue
			}
			if line != "" {
				out = append(out, line)
			}
			// Word alone too wide: hard-break it, keeping the last chunk as
			// the current line so following words can still append to it.
			if measure(face, word) > maxW {
				chunks := breakWord(face, word, maxW)
				out = append(out, chunks[:len(chunks)-1]...)
				line = chunks[len(chunks)-1]
			} else {
				line = word
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func breakWord(face font.Face, word string, maxW int) []string {
	var out []string
	cur := ""
	for _, r := range word {
		next := cur + string(r)
		if measure(face, next) > maxW && cur != "" {
			out = append(out, cur)
			cur = string(r)
		} else {
			cur = next
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
