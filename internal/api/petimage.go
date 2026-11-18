package api

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // decode uploaded GIFs
	_ "image/jpeg" // decode uploaded JPEGs
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/liftedkilt/openchore/internal/model"
)

// Kid-made accessories: a drawing or picture redeemed through a "design"
// pet-care reward. Stored as a small PNG under uploads/pets and worn as
// "img:<slot>:<id>".

const (
	petImageDir      = uploadDir + "/pets"
	petImageMaxSide  = 256
	petImageMaxBytes = 3 << 20 // decoded upload size
	petImageMaxPix   = 4096 * 4096
)

// decodePetImage turns a data: URL (PNG, JPEG or GIF) into a PNG no bigger
// than petImageMaxSide on its longest side.
func decodePetImage(dataURL string) ([]byte, error) {
	meta, payload, ok := strings.Cut(dataURL, ",")
	if !ok || !strings.HasPrefix(meta, "data:image/") || !strings.HasSuffix(meta, ";base64") {
		return nil, errors.New("send the picture as an image data URL")
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(raw) == 0 {
		return nil, errors.New("couldn't read that picture")
	}
	if len(raw) > petImageMaxBytes {
		return nil, errors.New("that picture is too big")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 || cfg.Width*cfg.Height > petImageMaxPix {
		return nil, errors.New("couldn't read that picture")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("couldn't read that picture")
	}
	out := shrink(src, petImageMaxSide)
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// shrink box-filters src so its longest side is at most max.
func shrink(src image.Image, max int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := 1.0
	if w > max || h > max {
		scale = float64(max) / float64(w)
		if h > w {
			scale = float64(max) / float64(h)
		}
	}
	dw, dh := int(float64(w)*scale+0.5), int(float64(h)*scale+0.5)
	dw, dh = maxInt(dw, 1), maxInt(dh, 1)
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		y0, y1 := b.Min.Y+y*h/dh, b.Min.Y+maxInt((y+1)*h/dh, y*h/dh+1)
		for x := 0; x < dw; x++ {
			x0, x1 := b.Min.X+x*w/dw, b.Min.X+maxInt((x+1)*w/dw, x*w/dw+1)
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					c := color.NRGBAModel.Convert(src.At(sx, sy)).(color.NRGBA)
					// Weight colour by alpha so transparent edges don't darken.
					r += uint64(c.R) * uint64(c.A)
					g += uint64(c.G) * uint64(c.A)
					bl += uint64(c.B) * uint64(c.A)
					a += uint64(c.A)
					n++
				}
			}
			if a > 0 {
				dst.SetNRGBA(x, y, color.NRGBA{R: uint8(r / a), G: uint8(g / a), B: uint8(bl / a), A: uint8(a / n)})
			}
		}
	}
	return dst
}

// Custom animal pictures (docs/custom-animals.md): a small PNG with a
// transparent background, stored as it was drawn so pixel art stays crisp.
// It is either one picture or a strip of up to spriteMaxFrames equal square
// frames side by side (model.PetSpriteFrames).
const (
	spriteMaxSide   = 128 // of a frame
	spriteMinSide   = 8
	spriteMaxFrames = 6
	spriteMaxBytes  = 2 << 20
	spriteMaxPix    = spriteMaxFrames * 1024 * 1024 // decoded before scaling: bounds the memory an upload can ask for
)

// spriteFrames says how many frames a w x h picture holds: a picture whose
// width is exactly 2 to 6 times its height is a strip of square frames;
// anything else is a single picture.
func spriteFrames(w, h int) int {
	if h > 0 && w%h == 0 && w/h >= 2 && w/h <= spriteMaxFrames {
		return w / h
	}
	return 1
}

// decodeSprite checks a data: URL holding a PNG and returns it re-encoded,
// with its number of frames. A picture bigger than spriteMaxSide (per frame)
// is scaled down by picking pixels (no smoothing), which keeps enlarged
// pixel art sharp.
func decodeSprite(dataURL string) ([]byte, int, error) {
	meta, payload, ok := strings.Cut(dataURL, ",")
	if !ok || meta != "data:image/png;base64" {
		return nil, 0, errors.New("the picture must be a PNG file")
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(raw) == 0 {
		return nil, 0, errors.New("couldn't read that picture")
	}
	if len(raw) > spriteMaxBytes {
		return nil, 0, errors.New("that picture is too big; make each frame 128 by 128 pixels or smaller")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width*cfg.Height > spriteMaxPix {
		return nil, 0, errors.New("couldn't read that picture; save it as a PNG")
	}
	if cfg.Width < spriteMinSide || cfg.Height < spriteMinSide {
		return nil, 0, errors.New("that picture is too small; 64 by 64 pixels works best")
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, errors.New("couldn't read that picture; save it as a PNG")
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	frames := spriteFrames(w, h)
	dw, dh := w, h
	if frames > 1 {
		// Frames are square: only the height decides the scale, and the
		// width stays an exact multiple of it.
		if h > spriteMaxSide {
			dw, dh = frames*spriteMaxSide, spriteMaxSide
		}
	} else if longest := max(w, h); longest > spriteMaxSide {
		dw, dh = max(1, w*spriteMaxSide/longest), max(1, h*spriteMaxSide/longest)
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// savePetImage writes a PNG and returns its id.
func savePetImage(pngData []byte) (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b[:])
	if err := os.MkdirAll(petImageDir, 0o750); err != nil {
		return "", err
	}
	return id, os.WriteFile(filepath.Join(petImageDir, id+".png"), pngData, 0o640)
}

// removePetFiles deletes every picture that belongs to a pet: its own
// animal picture and any accessories it drew or uploaded.
func removePetFiles(p *model.Pet) {
	if p == nil {
		return
	}
	if p.Sprite != "" {
		removePetImage(p.Sprite)
	}
	for _, it := range p.Items {
		if rest, ok := strings.CutPrefix(it, "img:"); ok {
			if _, id, ok := strings.Cut(rest, ":"); ok && model.ValidPetItem(it) {
				removePetImage(id)
			}
		}
	}
}

func removePetImage(id string) {
	_ = os.Remove(filepath.Join(petImageDir, id+".png"))
}
