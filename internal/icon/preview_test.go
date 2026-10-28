package icon

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"
)

// TestPreview writes a contact sheet of the icon variants when ICON_PREVIEW is set.
func TestPreview(t *testing.T) {
	out := os.Getenv("ICON_PREVIEW")
	if out == "" {
		t.Skip("set ICON_PREVIEW=path.png")
	}
	sheet := image.NewNRGBA(image.Rect(0, 0, 720, 300))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{color.NRGBA{40, 44, 52, 255}}, image.Point{}, draw.Src)
	draw.Draw(sheet, image.Rect(360, 0, 720, 300), &image.Uniform{color.NRGBA{243, 246, 250, 255}}, image.Point{}, draw.Src)
	x := 10
	for _, d := range []*color.NRGBA{nil, &Green, &Red} {
		img := Draw(96, d)
		draw.Draw(sheet, image.Rect(x, 10, x+96, 106), img, image.Point{}, draw.Over)
		draw.Draw(sheet, image.Rect(x+360, 10, x+456, 106), img, image.Point{}, draw.Over)
		for i, s := range []int{16, 24, 32} {
			small := Draw(s, d)
			draw.Draw(sheet, image.Rect(x+i*36, 130, x+i*36+s, 130+s), small, image.Point{}, draw.Over)
			draw.Draw(sheet, image.Rect(x+360+i*36, 130, x+360+i*36+s, 130+s), small, image.Point{}, draw.Over)
		}
		x += 115
	}
	big := Draw(120, nil)
	draw.Draw(sheet, image.Rect(10, 175, 130, 295), big, image.Point{}, draw.Over)
	f, _ := os.Create(out)
	defer f.Close()
	png.Encode(f, sheet)
}
