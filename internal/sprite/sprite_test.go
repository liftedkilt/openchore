package sprite

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const maxFriendPictureBytes = 48 << 10 // mirrors internal/friends/friends.go maxPictureBytes

func TestStarterPackSprites(t *testing.T) {
	packDir := filepath.Join("..", "..", "web", "public", "sprites", "pack")
	matches, err := filepath.Glob(filepath.Join(packDir, "*.png"))
	if err != nil {
		t.Fatalf("glob starter pack: %v", err)
	}
	var got []string
	for _, m := range matches {
		got = append(got, filepath.Base(m))
	}
	slices.Sort(got)

	want := []string{
		"alpaca-strip.png",
		"donkey-strip.png",
		"duck-strip.png",
		"goose-strip.png",
		"mouse-strip.png",
		"rabbit-strip.png",
		"rooster-strip.png",
		"turkey-strip.png",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("starter pack files = %v, want %v", got, want)
	}

	const frameSize = 64
	const wantFrames = 6
	const wantWidth = frameSize * wantFrames

	for _, name := range want {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(packDir, name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if len(raw) > maxFriendPictureBytes {
				t.Errorf("raw size = %d bytes, exceeds friend-sharing limit %d", len(raw), maxFriendPictureBytes)
			}

			cfg, err := png.DecodeConfig(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("png.DecodeConfig: %v", err)
			}
			if cfg.Width != wantWidth || cfg.Height != frameSize {
				t.Fatalf("dimensions = %dx%d, want %dx%d", cfg.Width, cfg.Height, wantWidth, frameSize)
			}

			clean, frames, err := Decode(raw)
			if err != nil {
				t.Fatalf("Decode(%s) failed: %v", name, err)
			}
			if frames != wantFrames {
				t.Fatalf("Decode frames = %d, want %d", frames, wantFrames)
			}
			if len(clean) > maxFriendPictureBytes {
				t.Errorf("re-encoded size = %d bytes, exceeds friend-sharing limit %d", len(clean), maxFriendPictureBytes)
			}

			img, err := png.Decode(bytes.NewReader(clean))
			if err != nil {
				t.Fatalf("png.Decode(clean): %v", err)
			}
			b := img.Bounds()
			if b.Dx() != wantWidth || b.Dy() != frameSize {
				t.Fatalf("re-encoded dimensions = %dx%d, want %dx%d", b.Dx(), b.Dy(), wantWidth, frameSize)
			}

			uniqueFrames := make(map[[sha256.Size]byte]int)
			for f := 0; f < wantFrames; f++ {
				x0 := b.Min.X + f*frameSize
				var solid, clear, grounded int
				frameBuf := make([]byte, 0, frameSize*frameSize*4)

				for y := 0; y < frameSize; y++ {
					for x := 0; x < frameSize; x++ {
						c := color.NRGBAModel.Convert(img.At(x0+x, b.Min.Y+y)).(color.NRGBA)
						if c.A != 0 && c.A != 255 {
							t.Errorf("frame %d pixel (%d,%d) has semi-transparent alpha %d (want 0 or 255)", f, x, y, c.A)
						}
						if (x == 0 || x == frameSize-1) && c.A != 0 {
							t.Errorf("frame %d pixel (%d,%d) touches horizontal frame edge (alpha %d)", f, x, y, c.A)
						}
						if c.A < 16 {
							clear++
						} else {
							solid++
							if y == frameSize-1 {
								grounded++
							}
						}
						frameBuf = append(frameBuf, c.R, c.G, c.B, c.A)
					}
				}

				if clear == 0 {
					t.Errorf("frame %d has no transparent pixels", f)
				}
				if solid == 0 {
					t.Errorf("frame %d is completely blank", f)
				}
				topLeft := color.NRGBAModel.Convert(img.At(x0, b.Min.Y)).(color.NRGBA)
				topRight := color.NRGBAModel.Convert(img.At(x0+frameSize-1, b.Min.Y)).(color.NRGBA)
				if topLeft.A != 0 || topRight.A != 0 {
					t.Errorf("frame %d top corners must be transparent (left=%d, right=%d)", f, topLeft.A, topRight.A)
				}
				if grounded == 0 {
					t.Errorf("frame %d has no opaque pixels on bottom ground row y=%d", f, frameSize-1)
				}
				uniqueFrames[sha256.Sum256(frameBuf)] = f
			}

			if len(uniqueFrames) != wantFrames {
				t.Errorf("got %d distinct frames, want %d", len(uniqueFrames), wantFrames)
			}
		})
	}
}

func TestFrames(t *testing.T) {
	cases := []struct {
		w, h int
		want int
	}{
		{64, 64, 1},
		{128, 64, 2},
		{192, 64, 3},
		{384, 64, 6},
		{448, 64, 1}, // 7 > MaxFrames
		{100, 64, 1}, // not an exact multiple
		{64, 0, 1},   // zero height
		{0, 64, 1},   // zero width
	}
	for _, tc := range cases {
		if got := Frames(tc.w, tc.h); got != tc.want {
			t.Errorf("Frames(%d, %d) = %d, want %d", tc.w, tc.h, got, tc.want)
		}
	}
}

func encodeTestPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func TestDecodeEdgeCases(t *testing.T) {
	red := color.NRGBA{R: 220, G: 60, B: 50, A: 255}

	t.Run("empty input", func(t *testing.T) {
		if _, _, err := Decode(nil); err == nil {
			t.Fatal("expected error for empty input")
		}
	})

	t.Run("invalid png", func(t *testing.T) {
		if _, _, err := Decode([]byte("not a png")); err == nil {
			t.Fatal("expected error for non-PNG data")
		}
	})

	t.Run("too small", func(t *testing.T) {
		im := image.NewNRGBA(image.Rect(0, 0, MinSide-1, MinSide-1))
		im.SetNRGBA(1, 1, red)
		if _, _, err := Decode(encodeTestPNG(t, im)); err == nil {
			t.Fatal("expected error for image smaller than MinSide")
		}
	})

	t.Run("opaque background rejected", func(t *testing.T) {
		im := image.NewNRGBA(image.Rect(0, 0, 16, 16))
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				im.SetNRGBA(x, y, red)
			}
		}
		if _, _, err := Decode(encodeTestPNG(t, im)); err == nil {
			t.Fatal("expected error for fully opaque image")
		}
	})

	t.Run("blank single image rejected", func(t *testing.T) {
		im := image.NewNRGBA(image.Rect(0, 0, 16, 16))
		if _, _, err := Decode(encodeTestPNG(t, im)); err == nil {
			t.Fatal("expected error for empty single picture")
		}
	})

	t.Run("blank frame 0 in strip rejected", func(t *testing.T) {
		im := image.NewNRGBA(image.Rect(0, 0, 32, 16))
		im.SetNRGBA(20, 8, red) // only frame 1 drawn
		if _, _, err := Decode(encodeTestPNG(t, im)); err == nil {
			t.Fatal("expected error when frame 0 is blank")
		}
	})

	t.Run("middle blank frame rejected", func(t *testing.T) {
		im := image.NewNRGBA(image.Rect(0, 0, 48, 16)) // 3 frames
		im.SetNRGBA(4, 8, red)                         // frame 0
		im.SetNRGBA(36, 8, red)                        // frame 2 (frame 1 blank)
		if _, _, err := Decode(encodeTestPNG(t, im)); err == nil {
			t.Fatal("expected error when middle frame is blank")
		}
	})

	t.Run("trailing blank frames trimmed", func(t *testing.T) {
		im := image.NewNRGBA(image.Rect(0, 0, 6*16, 16)) // 6 frames
		im.SetNRGBA(4, 8, red)                           // frame 0
		im.SetNRGBA(20, 8, red)                          // frame 1
		im.SetNRGBA(36, 8, red)                          // frame 2
		clean, frames, err := Decode(encodeTestPNG(t, im))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if frames != 3 {
			t.Fatalf("frames = %d, want 3", frames)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(clean))
		if err != nil {
			t.Fatalf("DecodeConfig: %v", err)
		}
		if cfg.Width != 48 || cfg.Height != 16 {
			t.Fatalf("trimmed dimensions = %dx%d, want 48x16", cfg.Width, cfg.Height)
		}
	})

	t.Run("oversized strip downscaled preserving square frames", func(t *testing.T) {
		// 4 frames of 256x256 -> downscaled to 4 frames of 128x128 (512x128).
		im := image.NewNRGBA(image.Rect(0, 0, 4*256, 256))
		for f := 0; f < 4; f++ {
			for y := 64; y < 192; y++ {
				for x := 64; x < 192; x++ {
					im.SetNRGBA(f*256+x, y, red)
				}
			}
		}
		clean, frames, err := Decode(encodeTestPNG(t, im))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if frames != 4 {
			t.Fatalf("frames = %d, want 4", frames)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(clean))
		if err != nil {
			t.Fatalf("DecodeConfig: %v", err)
		}
		if cfg.Width != 4*MaxSide || cfg.Height != MaxSide {
			t.Fatalf("scaled dimensions = %dx%d, want %dx%d", cfg.Width, cfg.Height, 4*MaxSide, MaxSide)
		}
	})

	t.Run("oversized single picture downscaled", func(t *testing.T) {
		im := image.NewNRGBA(image.Rect(0, 0, 256, 200))
		for y := 40; y < 140; y++ {
			for x := 40; x < 140; x++ {
				im.SetNRGBA(x, y, red)
			}
		}
		clean, frames, err := Decode(encodeTestPNG(t, im))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if frames != 1 {
			t.Fatalf("frames = %d, want 1", frames)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(clean))
		if err != nil {
			t.Fatalf("DecodeConfig: %v", err)
		}
		if cfg.Width != MaxSide || cfg.Height != 100 {
			t.Fatalf("scaled dimensions = %dx%d, want %dx100", cfg.Width, cfg.Height, MaxSide)
		}
	})
}
