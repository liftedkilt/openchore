package api

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	_ "image/gif"  // decode uploaded GIFs
	_ "image/jpeg" // decode uploaded JPEGs
	"image/png"
	"os"
	"path/filepath"
	"strings"
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

func removePetImage(id string) {
	_ = os.Remove(filepath.Join(petImageDir, id+".png"))
}
