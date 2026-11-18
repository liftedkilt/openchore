// Package sprite checks custom animal pictures (docs/custom-animals.md): a
// small PNG with a transparent background, kept as it was drawn so pixel art
// stays crisp. A picture is either one image or a strip of up to MaxFrames
// equal square frames side by side (model.PetSpriteFrames).
//
// The same check runs on a picture a kid uploads and on one that arrives
// from a friend family's server, which is untrusted input.
package sprite

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

const (
	MaxSide   = 128 // of a frame
	MinSide   = 8
	MaxFrames = 6
	MaxBytes  = 2 << 20
	maxPix    = MaxFrames * 1024 * 1024 // decoded before scaling: bounds the memory a picture can ask for
)

// Frames says how many frames a w x h picture holds: a picture whose width
// is exactly 2 to MaxFrames times its height is a strip of square frames;
// anything else is a single picture.
func Frames(w, h int) int {
	if h > 0 && w%h == 0 && w/h >= 2 && w/h <= MaxFrames {
		return w / h
	}
	return 1
}

// Decode checks PNG data and returns it re-encoded, with its number of
// frames. A picture bigger than MaxSide (per frame) is scaled down by
// picking pixels (no smoothing), which keeps enlarged pixel art sharp.
func Decode(raw []byte) ([]byte, int, error) {
	if len(raw) == 0 {
		return nil, 0, errors.New("couldn't read that picture")
	}
	if len(raw) > MaxBytes {
		return nil, 0, errors.New("that picture is too big; make each frame 128 by 128 pixels or smaller")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width*cfg.Height > maxPix {
		return nil, 0, errors.New("couldn't read that picture; save it as a PNG")
	}
	if cfg.Width < MinSide || cfg.Height < MinSide {
		return nil, 0, errors.New("that picture is too small; 64 by 64 pixels works best")
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, errors.New("couldn't read that picture; save it as a PNG")
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	frames := Frames(w, h)
	dw, dh := w, h
	if frames > 1 {
		// Frames are square: only the height decides the scale, and the
		// width stays an exact multiple of it.
		if h > MaxSide {
			dw, dh = frames*MaxSide, MaxSide
		}
	} else if longest := max(w, h); longest > MaxSide {
		dw, dh = max(1, w*MaxSide/longest), max(1, h*MaxSide/longest)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	var clear int
	solid := make([]int, frames) // per frame
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			// The middle of each target pixel's patch in the source.
			c := color.NRGBAModel.Convert(src.At(b.Min.X+(2*x+1)*w/(2*dw), b.Min.Y+(2*y+1)*h/(2*dh))).(color.NRGBA)
			dst.SetNRGBA(x, y, c)
			if c.A < 16 {
				clear++
			} else {
				solid[x*frames/dw]++
			}
		}
	}
	switch {
	case solid[0] == 0 && frames == 1:
		return nil, 0, errors.New("that picture is empty")
	case solid[0] == 0:
		return nil, 0, errors.New("the first frame (standing) is empty")
	case clear == 0:
		return nil, 0, errors.New("the picture needs a see-through background, or the animal will be a square")
	}
	// A strip may leave later frames blank: stop at the last one drawn, so a
	// blank frame falls back to the standing one instead of showing nothing.
	strip := frames > 1
	for frames > 1 && solid[frames-1] == 0 {
		frames--
	}
	// ...but not one in the middle: the app would show an empty picture
	// for that mood instead of falling back.
	for i := 1; i < frames; i++ {
		if solid[i] == 0 {
			return nil, 0, fmt.Errorf("picture %d in the row is empty; draw every picture up to the last one you use, or leave the end of the row blank", i+1)
		}
	}
	if strip && frames*dh < dw {
		cut := image.NewNRGBA(image.Rect(0, 0, frames*dh, dh))
		for y := 0; y < dh; y++ {
			for x := 0; x < frames*dh; x++ {
				cut.SetNRGBA(x, y, dst.NRGBAAt(x, y))
			}
		}
		dst = cut
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), frames, nil
}
