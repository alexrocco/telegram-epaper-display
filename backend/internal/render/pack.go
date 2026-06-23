package render

import "image"

// Panel native dimensions (portrait), as expected by the Waveshare driver's
// 1-bit (1Gray) MONO_HLSB framebuffer.
const (
	PanelW = 280
	PanelH = 480
	// BufferSize is the exact size of the 1-bit framebuffer in bytes
	// (280*480/8). The firmware pushes this verbatim to the panel via the
	// 1Gray full-refresh mode, which is sharp (no grayscale ghosting).
	BufferSize = PanelW * PanelH / 8
)

// blackThreshold is the cutoff for bi-level (pure black/white) rendering.
// We render text in 1-bit black/white: gray levels on e-ink are physically
// muddy, so anti-aliased glyph edges quantized to gray look blurry. Raise it to
// make text heavier, lower it to make it lighter.
const blackThreshold = 128

// isWhite reports whether an 8-bit gray value renders as a white pixel.
func isWhite(y uint8) bool { return y >= blackThreshold }

// Pack converts the landscape CanvasW x CanvasH image into the panel's native
// 280x480 MONO_HLSB 1-bit framebuffer (BufferSize bytes). In this format a set
// bit is white and a clear bit is black; within each byte the leftmost pixel is
// the most-significant bit.
//
// The landscape canvas is rotated 90° clockwise into the portrait panel:
// panel pixel (px, py) maps to canvas pixel (py, CanvasH-1-px). If the physical
// display ever shows the image upside down, flip the two mappings below.
func Pack(img *image.Gray) []byte {
	buf := make([]byte, BufferSize)
	const bytesPerRow = PanelW / 8
	for py := 0; py < PanelH; py++ {
		lx := py // canvas x
		rowBase := py * bytesPerRow
		for px := 0; px < PanelW; px++ {
			ly := (CanvasH - 1) - px // canvas y
			if isWhite(img.GrayAt(lx, ly).Y) {
				buf[rowBase+px/8] |= 1 << uint(7-(px&7))
			}
		}
	}
	return buf
}

// PreviewImage returns a copy of img quantized exactly like Pack (pure
// black/white), so a PNG preview matches what the device will actually show.
func PreviewImage(img *image.Gray) *image.Gray {
	out := image.NewGray(img.Bounds())
	for i, y := range img.Pix {
		if isWhite(y) {
			out.Pix[i] = 255
		} else {
			out.Pix[i] = 0
		}
	}
	return out
}
