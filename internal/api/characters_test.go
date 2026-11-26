package api_test

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/sprite/characters"
)

func (e *testEnv) characterRewardID(t *testing.T, userID int, id string) (int, int) {
	t.Helper()
	resp := e.expectStatus(t, "GET", "/api/rewards", nil, childHeaders(userID), http.StatusOK)
	var rewards []map[string]any
	decodeBody(t, resp, &rewards)
	for _, r := range rewards {
		if r["pet_action"] == model.PetActionCharacter && r["pet_item"] == id {
			return int(r["id"].(float64)), int(r["effective_cost"].(float64))
		}
	}
	t.Fatalf("no %s reward", id)
	return 0, 0
}

func TestPetCharacterReward(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.enablePets(t)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Puff", "species": "sheep"}, childHeaders(kid), http.StatusCreated)
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": kid, "amount": 500, "note": "seed"}, adminHeaders(), http.StatusNoContent)
	file := func(id string) string { return "data/uploads/pets/" + id + ".png" }

	// The picture is public, like the farm; unknown ones aren't there.
	resp := env.expectStatus(t, "GET", "/api/pets/characters/dragon.png", nil, nil, http.StatusOK)
	served, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if want, _ := characters.PNG("dragon"); string(served) != string(want) || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("served %d bytes as %q", len(served), resp.Header.Get("Content-Type"))
	}
	env.expectStatus(t, "GET", "/api/pets/characters/kraken.png", nil, nil, http.StatusNotFound).Body.Close()

	// Redeeming needs no pick: it charges the reward's own price and the
	// animal becomes the dragon, a six-frame picture.
	dragon, cost := env.characterRewardID(t, kid, "dragon")
	if cost != 100 {
		t.Fatalf("dragon costs %d", cost)
	}
	before := env.balance(t, kid)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", dragon), nil, childHeaders(kid), http.StatusCreated).Body.Close()
	if got := env.balance(t, kid); got != before-cost {
		t.Fatalf("balance %d, want %d", got, before-cost)
	}
	p := env.myPet(t, kid)
	first, _ := p["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(first)) })
	if first == "" || p["character"] != "dragon" || p["sprite_frames"] != 6.0 || p["species"] != "sheep" {
		t.Fatalf("pet: %v", p)
	}

	// Again is refused before any points move.
	before = env.balance(t, kid)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", dragon), nil, childHeaders(kid), http.StatusUnprocessableEntity).Body.Close()

	// The cheaper "own picture" reward won't take a copy of a character.
	picture := env.petRewardID(t, kid, "picture")
	raw, _ := characters.PNG("unicorn")
	copied := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", picture), map[string]any{"pet": map[string]any{"image": copied}}, childHeaders(kid), http.StatusUnprocessableEntity).Body.Close()
	if env.balance(t, kid) != before {
		t.Fatal("a refused redemption cost points")
	}

	// Another character replaces the first; an upload clears the character.
	unicorn, _ := env.characterRewardID(t, kid, "unicorn")
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", unicorn), nil, childHeaders(kid), http.StatusCreated).Body.Close()
	p = env.myPet(t, kid)
	second, _ := p["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(second)) })
	if p["character"] != "unicorn" {
		t.Fatalf("pet: %v", p)
	}
	if _, err := os.Stat(file(first)); !os.IsNotExist(err) {
		t.Fatal("the replaced picture was left on disk")
	}
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", picture), map[string]any{"pet": map[string]any{"image": spritePNG(t, 64, 64, false, 1)}}, childHeaders(kid), http.StatusCreated).Body.Close()
	p = env.myPet(t, kid)
	third, _ := p["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(third)) })
	if p["character"] != nil {
		t.Fatalf("an upload kept the character: %v", p)
	}

	// A character is the app's own art, so it goes to friend families
	// without waiting for a parent; an upload still waits.
	if p["sprite_approved"] != nil {
		t.Fatalf("an upload was approved for friends: %v", p)
	}
	redeemID := func(reward int) int64 {
		t.Helper()
		resp := env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", reward), nil, childHeaders(kid), http.StatusCreated)
		var out map[string]any
		decodeBody(t, resp, &out)
		return int64(out["id"].(float64))
	}
	griffin, griffinCost := env.characterRewardID(t, kid, "griffin")
	paid := env.balance(t, kid)
	gr := redeemID(griffin)
	p = env.myPet(t, kid)
	fourth, _ := p["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(fourth)) })
	if p["character"] != "griffin" || p["sprite_approved"] != true {
		t.Fatalf("griffin: %v", p)
	}

	// Undoing an older character purchase refunds it but leaves the griffin.
	yeti, _ := env.characterRewardID(t, kid, "yeti")
	yt := redeemID(yeti)
	p = env.myPet(t, kid)
	fifth, _ := p["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(fifth)) })
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/redemptions/%d", gr), nil, adminHeaders(), http.StatusNoContent).Body.Close()
	if p := env.myPet(t, kid); p["character"] != "yeti" {
		t.Fatalf("undoing an older purchase changed the animal: %v", p)
	}
	// Undoing the purchase the animal still shows puts back the drawn
	// animal and deletes the picture, as well as refunding.
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/redemptions/%d", yt), nil, adminHeaders(), http.StatusNoContent).Body.Close()
	if p := env.myPet(t, kid); p["sprite"] != nil || p["character"] != nil {
		t.Fatalf("undo kept the character: %v", p)
	}
	if _, err := os.Stat(file(fifth)); !os.IsNotExist(err) {
		t.Fatal("the undone character's picture was left on disk")
	}
	if got := env.balance(t, kid); got != paid {
		t.Fatalf("balance after both undos %d, want %d (griffin cost %d)", got, paid, griffinCost)
	}

	// Two taps at once buy it once.
	phoenix, _ := env.characterRewardID(t, kid, "phoenix")
	codes := make(chan int, 2)
	for range 2 {
		go func() {
			resp := env.request(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", phoenix), nil, childHeaders(kid))
			resp.Body.Close()
			codes <- resp.StatusCode
		}()
	}
	if a, b := <-codes, <-codes; a+b != http.StatusCreated+http.StatusUnprocessableEntity {
		t.Fatalf("two taps: %d and %d", a, b)
	}
	p = env.myPet(t, kid)
	sixth, _ := p["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(sixth)) })

	// Parents can only make character rewards for real characters.
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{"name": "Kraken", "cost": 50, "pet_action": "character", "pet_item": "kraken"}, adminHeaders(), http.StatusBadRequest).Body.Close()
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{"name": "Golden dragon", "cost": 250, "pet_action": "character", "pet_item": "dragon"}, adminHeaders(), http.StatusCreated).Body.Close()
}
