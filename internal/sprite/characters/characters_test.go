package characters

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/sprite"
)

func encode(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decode(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// Every character in the model has a six-frame picture that passes the
// same check as an upload, and there are no pictures for unknown ones.
func TestEveryCharacterHasAPicture(t *testing.T) {
	for _, c := range model.PetCharacters {
		data, ok := PNG(c.ID)
		if !ok {
			t.Fatalf("%s: no picture", c.ID)
		}
		if _, frames, err := sprite.Decode(data); err != nil || frames != 6 {
			t.Fatalf("%s: frames=%d err=%v", c.ID, frames, err)
		}
	}
	entries, _ := fs.ReadDir(files, ".")
	if len(entries) != len(model.PetCharacters) {
		t.Fatalf("%d pictures for %d characters", len(entries), len(model.PetCharacters))
	}
	for _, e := range entries {
		if model.PetCharacterName(strings.TrimSuffix(e.Name(), ".png")) == "" {
			t.Fatalf("picture for an unknown character: %s", e.Name())
		}
	}
	if _, ok := PNG("../characters"); ok {
		t.Fatal("served a path that isn't a character")
	}
}

// A copy is recognised whole, enlarged, flipped or as one frame cut out.
func TestMatchFindsCopies(t *testing.T) {
	for _, c := range model.PetCharacters {
		data, _ := PNG(c.ID)
		strip := decode(t, data)
		b := strip.Bounds()

		big := image.NewNRGBA(image.Rect(0, 0, b.Dx()*2, b.Dy()*2))
		for y := 0; y < big.Bounds().Dy(); y++ {
			for x := 0; x < big.Bounds().Dx(); x++ {
				big.Set(x, y, strip.At(x/2, y/2))
			}
		}
		one := image.NewNRGBA(image.Rect(0, 0, b.Dy(), b.Dy()))
		draw.Draw(one, one.Bounds(), strip, image.Pt(b.Dy()*5, 0), draw.Src) // the happy frame
		flipped := image.NewNRGBA(one.Bounds())
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dy(); x++ {
				flipped.Set(b.Dy()-1-x, y, one.At(x, y))
			}
		}
		// Small edits: a few stray pixels, a recoloured patch, a shift.
		edited := image.NewNRGBA(one.Bounds())
		draw.Draw(edited, edited.Bounds(), one, image.Point{}, draw.Src)
		for _, p := range [][2]int{{10, 10}, {40, 50}, {30, 30}, {50, 20}} {
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					edited.Set(p[0]+dx, p[1]+dy, color.NRGBA{R: 255, G: 0, B: 255, A: 255})
				}
			}
		}
		// A slight recolour: every channel nudged.
		tinted := image.NewNRGBA(one.Bounds())
		for i := 0; i < len(tinted.Pix); i += 4 {
			for c := 0; c < 3; c++ {
				tinted.Pix[i+c] = uint8(min(255, int(one.Pix[i+c])+24))
			}
			tinted.Pix[i+3] = one.Pix[i+3]
		}
		shifted := image.NewNRGBA(one.Bounds())
		draw.Draw(shifted, shifted.Bounds().Add(image.Pt(-4, 2)), one, image.Point{}, draw.Src)
		for name, img := range map[string]image.Image{"whole": strip, "enlarged": big, "one frame": one, "flipped": flipped,
			"stray pixels": edited, "shifted": shifted, "recoloured": tinted} {
			if got := Match(encode(t, img)); got != c.ID {
				t.Errorf("%s %s: matched %q", c.ID, name, got)
			}
		}
	}

	// Something else entirely matches nothing.
	other := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 40; y < 64; y++ {
		for x := 10; x < 50; x++ {
			other.Pix[(y*64+x)*4], other.Pix[(y*64+x)*4+3] = 200, 255
		}
	}
	if got := Match(encode(t, other)); got != "" {
		t.Fatalf("a different picture matched %q", got)
	}
}

// None of the free pictures the app hands out (the starter pack, the sample
// and the drawing kit) is mistaken for a character.
func TestFreePicturesMatchNoCharacter(t *testing.T) {
	paths, _ := filepath.Glob("../../../web/public/sprites/*.png")
	pack, _ := filepath.Glob("../../../web/public/sprites/pack/*.png")
	paths = append(paths, pack...)
	if len(pack) < 8 {
		t.Fatalf("found %d starter pack pictures", len(pack))
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := Match(data); got != "" {
			t.Errorf("%s matched %s", filepath.Base(p), got)
		}
	}
}
