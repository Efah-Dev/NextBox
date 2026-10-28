// Package icon draws the NextBox pink cat paw, used for the app icon and the
// tray icon (optionally with a status dot in the lower right corner).
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

var (
	Green = color.NRGBA{R: 0x22, G: 0xC5, B: 0x5E, A: 0xFF}
	Red   = color.NRGBA{R: 0xEF, G: 0x44, B: 0x44, A: 0xFF}

	pinkTop    = [3]float64{0xFF, 0xA3, 0xC8}
	pinkBottom = [3]float64{0xFF, 0x5C, 0x9A}
)

type ellipse struct{ cx, cy, rx, ry, angle float64 }

func (e ellipse) contains(x, y float64) bool {
	dx, dy := x-e.cx, y-e.cy
	s, c := math.Sincos(-e.angle * math.Pi / 180)
	u := dx*c - dy*s
	v := dx*s + dy*c
	return (u*u)/(e.rx*e.rx)+(v*v)/(e.ry*e.ry) <= 1
}

// The paw in unit coordinates (y grows downwards): three overlapping shapes
// form the main pad, four tilted ellipses are the toe beans.
var paw = []ellipse{
	{0.500, 0.700, 0.190, 0.190, 0},
	{0.385, 0.640, 0.145, 0.145, 0},
	{0.615, 0.640, 0.145, 0.145, 0},
	{0.500, 0.675, 0.250, 0.170, 0},
	{0.200, 0.405, 0.085, 0.115, -28},
	{0.375, 0.230, 0.092, 0.125, -10},
	{0.625, 0.230, 0.092, 0.125, 10},
	{0.800, 0.405, 0.085, 0.115, 28},
}

// The paw is tilted 45° clockwise (toes towards the upper right) and scaled to
// keep the rotated shape inside the canvas.
const (
	tilt  = 45.0
	scale = 0.94
)

var tiltSin, tiltCos = math.Sincos(-tilt * math.Pi / 180)

func inPaw(x, y float64) bool {
	// map the canvas point back into the upright paw coordinates
	dx, dy := (x-0.5)/scale, (y-0.5)/scale
	x = 0.5 + dx*tiltCos - dy*tiltSin
	y = 0.5 + dx*tiltSin + dy*tiltCos
	for _, e := range paw {
		if e.contains(x, y) {
			return true
		}
	}
	return false
}

// Draw renders the paw at size×size pixels. When dot is not nil, a status dot
// is drawn in the lower right corner with a transparent gap around it.
func Draw(size int, dot *color.NRGBA) *image.NRGBA {
	const ss = 4 // supersampling per axis
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const dotX, dotY, dotR, gapR = 0.79, 0.79, 0.19, 0.26
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var pawCov, dotCov float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					d := math.Hypot(x-dotX, y-dotY)
					if dot != nil && d <= dotR {
						dotCov++
					} else if (dot == nil || d > gapR) && inPaw(x, y) {
						pawCov++
					}
				}
			}
			pawCov /= ss * ss
			dotCov /= ss * ss
			t := (float64(py) + 0.5) / float64(size)
			r := pinkTop[0] + (pinkBottom[0]-pinkTop[0])*t
			g := pinkTop[1] + (pinkBottom[1]-pinkTop[1])*t
			b := pinkTop[2] + (pinkBottom[2]-pinkTop[2])*t
			a := pawCov
			if dotCov > 0 {
				// dot and paw never overlap, so a plain mix is exact
				total := pawCov + dotCov
				r = (r*pawCov + float64(dot.R)*dotCov) / total
				g = (g*pawCov + float64(dot.G)*dotCov) / total
				b = (b*pawCov + float64(dot.B)*dotCov) / total
				a = total
			}
			img.SetNRGBA(px, py, color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(math.Round(a * 255))})
		}
	}
	return img
}

func PNG(size int, dot *color.NRGBA) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, Draw(size, dot))
	return buf.Bytes()
}

// ICO packs PNG renderings at the given sizes into a Windows .ico file.
func ICO(sizes []int, dot *color.NRGBA) []byte {
	images := make([][]byte, len(sizes))
	for i, s := range sizes {
		images[i] = PNG(s, dot)
	}
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		dim := uint8(s)
		if s >= 256 {
			dim = 0 // 0 means 256 in the ICO directory
		}
		binary.Write(&buf, binary.LittleEndian, struct {
			W, H, Colors, Reserved uint8
			Planes, BPP            uint16
			Size, Offset           uint32
		}{dim, dim, 0, 0, 1, 32, uint32(len(images[i])), uint32(offset)})
		offset += len(images[i])
	}
	for _, img := range images {
		buf.Write(img)
	}
	return buf.Bytes()
}
