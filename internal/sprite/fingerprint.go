package sprite

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// gridSize is the size each frame is sampled down to for comparing
// pictures. Ready-made pictures are drawn on a 32 x 32 grid and enlarged,
// so sampling the middle of each grid cell gives the same pixels at 32, 64
// or 128 pixels a side.
const gridSize = 32

const (
	// A copy may be shifted this many grid cells (2 pixels each at 64 x 64).
	maxShift = 2
	// Colours within this much on each channel count as the same, so a
	// re-save or a slight recolour still matches.
	colorTolerance = 40
	// Share of the solid cells that must line up for two frames to be the
	// same picture: a few changed cells don't make a new drawing.
	minOverlap = 0.85
)

type cell struct {
	r, g, b uint8
	solid   bool
}

// Grid is one frame of a picture, sampled to a fixed grid for comparing.
type Grid struct {
	cells [gridSize * gridSize]cell
	solid int
}

// Grids samples each non-blank frame of a PNG picture (a strip's frames, or
// the one picture) to a Grid.
func Grids(pngData []byte) ([]Grid, error) {
	src, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	frames := Frames(b.Dx(), b.Dy())
	fw := b.Dx() / frames
	var out []Grid
	for f := 0; f < frames; f++ {
		if g := sampleGrid(src, b.Min.X+f*fw, b.Min.Y, fw, b.Dy()); g.solid > 0 {
			out = append(out, g)
		}
	}
	return out, nil
}

func sampleGrid(src image.Image, x0, y0, w, h int) Grid {
	var g Grid
	for y := 0; y < gridSize; y++ {
		for x := 0; x < gridSize; x++ {
			c := color.NRGBAModel.Convert(src.At(x0+(2*x+1)*w/(2*gridSize), y0+(2*y+1)*h/(2*gridSize))).(color.NRGBA)
			if c.A >= 128 {
				g.cells[y*gridSize+x] = cell{c.R, c.G, c.B, true}
				g.solid++
			}
		}
	}
	return g
}

// Copies reports whether frame a is a copy of frame b: the same drawing,
// perhaps flipped, shifted a little, slightly recoloured or with a few
// cells changed.
func Copies(a, b Grid) bool {
	if a.solid == 0 || b.solid == 0 {
		return false
	}
	need := int(minOverlap * float64(max(a.solid, b.solid)))
	// Far more or fewer solid cells can't reach the overlap at any shift.
	if min(a.solid, b.solid) < need {
		return false
	}
	for _, flip := range []bool{false, true} {
		for dy := -maxShift; dy <= maxShift; dy++ {
			for dx := -maxShift; dx <= maxShift; dx++ {
				if overlap(a, b, dx, dy, flip, need) >= need {
					return true
				}
			}
		}
	}
	return false
}

// overlap counts b's solid cells that a has, in a close colour, at the
// same place once a is flipped and moved by (dx, dy). It stops early once
// need can no longer be reached.
func overlap(a, b Grid, dx, dy int, flip bool, need int) int {
	n, left := 0, b.solid
	for y := 0; y < gridSize; y++ {
		for x := 0; x < gridSize; x++ {
			bc := b.cells[y*gridSize+x]
			if !bc.solid {
				continue
			}
			left--
			ax, ay := x+dx, y+dy
			if flip {
				ax = gridSize - 1 - x + dx
			}
			if ax >= 0 && ax < gridSize && ay >= 0 && ay < gridSize {
				if ac := a.cells[ay*gridSize+ax]; ac.solid && near(ac.r, bc.r) && near(ac.g, bc.g) && near(ac.b, bc.b) {
					n++
				}
			}
			if n+left < need {
				return n
			}
		}
	}
	return n
}

func near(a, b uint8) bool {
	d := int(a) - int(b)
	return d >= -colorTolerance && d <= colorTolerance
}
