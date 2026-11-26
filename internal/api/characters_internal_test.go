package api

import (
	"testing"

	"github.com/liftedkilt/openchore/internal/model"
)

// The starter set has one reward per character, at its own price.
func TestDefaultPetRewardsHaveEveryCharacter(t *testing.T) {
	seen := map[string]bool{}
	for _, rw := range defaultPetRewards {
		if rw.PetAction == model.PetActionCharacter {
			if model.PetCharacterName(rw.PetItem) == "" {
				t.Fatalf("unknown character %q", rw.PetItem)
			}
			seen[rw.PetItem] = true
		}
	}
	if len(seen) != len(model.PetCharacters) {
		t.Fatalf("rewards for %d of %d characters", len(seen), len(model.PetCharacters))
	}
}
