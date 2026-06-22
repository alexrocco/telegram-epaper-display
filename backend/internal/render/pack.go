package render

import "image"

// Panel native dimensions (portrait), as expected by the Waveshare driver's
// GS2_HMSB framebuffer.
const (
	PanelW = 280
	PanelH = 480
	// BufferSize is the exact size of the 4-gray framebuffer in bytes
	// (280*480/4). The firmware pushes this verbatim to the panel.
	BufferSize = PanelW * PanelH / 4
)

// levelOf quantizes an 8-bit gray value to the panel's 2-bit level.
// 0=black, 1=dark gray, 2=light gray, 3=white (matches the driver mapping).
func levelOf(y uint8) byte { return byte(y >> 6) }

// Pack converts the landscape CanvasW x CanvasH image into the panel's native
// 280x480 GS2_HMSB framebuffer (BufferSize bytes).
//
// The landscape canvas is rotated 90° clockwise into the portrait panel:
// panel pixel (px, py) maps to canvas pixel (py, CanvasH-1-px). If the physical
// display shows the image upside down, flip the two mappings below.
func Pack(img *image.Gray) []byte {
	buf := make([]byte, BufferSize)
	const bytesPerRow = PanelW / 4
	for py := 0; py < PanelH; py++ {
		lx := py // canvas x
		rowBase := py * bytesPerRow
		for px := 0; px < PanelW; px++ {
			ly := (CanvasH - 1) - px // canvas y
			level := levelOf(img.GrayAt(lx, ly).Y)
			shift := uint((3 - (px & 3)) * 2)
			buf[rowBase+px/4] |= level << shift
		}
	}
	return buf
}

// PreviewImage returns a copy of img quantized to the panel's 4 shades, so a
// PNG preview matches what the device will actually show.
func PreviewImage(img *image.Gray) *image.Gray {
	shades := [4]uint8{0, 85, 170, 255}
	out := image.NewGray(img.Bounds())
	for i, y := range img.Pix {
		out.Pix[i] = shades[y>>6]
	}
	return out
}
