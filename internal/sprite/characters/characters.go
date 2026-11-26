// Package characters holds the ready-made character pictures that
// "character" pet-care rewards turn an animal into (model.PetCharacters,
// docs/custom-animals.md). They are drawn by web/scripts/gen-fantasy-pack.py
// and built into the server, which hands them out itself, so a kid can't
// get one by uploading a copy through the cheaper "picture" reward.
package characters

import (
	"embed"
	"sync"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/sprite"
)

//go:embed *.png
var files embed.FS

// PNG returns a character's picture strip, or false if id isn't one.
func PNG(id string) ([]byte, bool) {
	if model.PetCharacterName(id) == "" {
		return nil, false
	}
	data, err := files.ReadFile(id + ".png")
	return data, err == nil
}

var (
	refsOnce sync.Once
	refs     []ref
)

type ref struct {
	id   string
	grid sprite.Grid
}

// Match reports which character a picture copies, by any one of its frames,
// or "" if it copies none. A copy still counts when it is flipped, saved
// bigger, shifted a little, slightly recoloured or has a few cells changed
// (sprite.Copies); a kid's own drawing of a dragon doesn't.
func Match(pngData []byte) string {
	refsOnce.Do(func() {
		for _, c := range model.PetCharacters {
			data, ok := PNG(c.ID)
			if !ok {
				continue
			}
			grids, err := sprite.Grids(data)
			if err != nil {
				continue
			}
			for _, g := range grids {
				refs = append(refs, ref{c.ID, g})
			}
		}
	})
	grids, err := sprite.Grids(pngData)
	if err != nil {
		return ""
	}
	for _, g := range grids {
		for _, r := range refs {
			if sprite.Copies(g, r.grid) {
				return r.id
			}
		}
	}
	return ""
}
