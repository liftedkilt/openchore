package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/webhook"
)

func (e *testEnv) enablePets(t *testing.T) {
	t.Helper()
	e.expectStatus(t, "PUT", "/api/admin/pets", map[string]any{"enabled": true}, adminHeaders(), http.StatusOK)
}

func (e *testEnv) myPet(t *testing.T, userID int) map[string]any {
	t.Helper()
	resp := e.expectStatus(t, "GET", "/api/me/pet", nil, childHeaders(userID), http.StatusOK)
	var out map[string]any
	decodeBody(t, resp, &out)
	p, _ := out["pet"].(map[string]any)
	return p
}

func (e *testEnv) petRewardID(t *testing.T, userID int, action string) int {
	t.Helper()
	resp := e.expectStatus(t, "GET", "/api/rewards", nil, childHeaders(userID), http.StatusOK)
	var rewards []map[string]any
	decodeBody(t, resp, &rewards)
	for _, r := range rewards {
		if r["pet_action"] == action {
			return int(r["id"].(float64))
		}
	}
	t.Fatalf("no %s reward", action)
	return 0
}

func TestPetsOffByDefault(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Daisy", "species": "cow"}, childHeaders(kid), http.StatusForbidden)
	// Turning pets on seeds pet-care rewards; they stay hidden while off.
	env.enablePets(t)
	env.expectStatus(t, "PUT", "/api/admin/pets", map[string]any{"enabled": false}, adminHeaders(), http.StatusOK)
	resp := env.expectStatus(t, "GET", "/api/rewards", nil, childHeaders(kid), http.StatusOK)
	var rewards []map[string]any
	decodeBody(t, resp, &rewards)
	if len(rewards) != 0 {
		t.Fatalf("pet rewards visible while pets are off: %v", rewards)
	}
	// Turning on again doesn't add a second set.
	resp = env.expectStatus(t, "PUT", "/api/admin/pets", map[string]any{"enabled": true}, adminHeaders(), http.StatusOK)
	var out map[string]any
	decodeBody(t, resp, &out)
	if out["rewards_added"].(float64) != 0 {
		t.Fatalf("re-seeded rewards: %v", out)
	}
}

func TestPetAdoptFarmAndCare(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	sib := env.createChild(t, "Sib")
	env.enablePets(t)

	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Rex", "species": "dragon"}, childHeaders(kid), http.StatusBadRequest)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Biscuit", "species": "dog"}, childHeaders(kid), http.StatusCreated)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Again", "species": "cat"}, childHeaders(kid), http.StatusConflict)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Clover", "species": "horse"}, childHeaders(sib), http.StatusCreated)

	// Everyone sees the whole farm.
	resp := env.expectStatus(t, "GET", "/api/pets", nil, childHeaders(sib), http.StatusOK)
	var farm map[string]any
	decodeBody(t, resp, &farm)
	pets := farm["pets"].([]any)
	if len(pets) != 2 || pets[0].(map[string]any)["owner_name"] != "Kid" {
		t.Fatalf("farm: %v", farm)
	}

	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": kid, "amount": 100, "note": "seed"}, adminHeaders(), http.StatusNoContent)

	feed := env.petRewardID(t, kid, "feed")
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", feed), nil, childHeaders(kid), http.StatusCreated)
	if p := env.myPet(t, kid); p["fullness"].(float64) != 100 {
		t.Fatalf("feeding: %v", p)
	}

	// Accessories: bought once, then worn or taken off for free.
	hat := env.petRewardID(t, kid, "accessory")
	env.expectStatus(t, "PATCH", "/api/me/pet", map[string]any{"accessory": "straw_hat"}, childHeaders(kid), http.StatusUnprocessableEntity)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", hat), nil, childHeaders(kid), http.StatusCreated)
	p := env.myPet(t, kid)
	if p["accessory"] == "" || len(p["items"].([]any)) != 1 {
		t.Fatalf("accessory: %v", p)
	}
	before := env.balance(t, kid)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", hat), nil, childHeaders(kid), http.StatusUnprocessableEntity)
	if env.balance(t, kid) != before {
		t.Fatal("refused accessory still cost points")
	}
	env.expectStatus(t, "PATCH", "/api/me/pet", map[string]any{"accessory": ""}, childHeaders(kid), http.StatusOK)

	// Pet rewards can't be shared or saved for.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/commit", feed), nil, childHeaders(kid), http.StatusUnprocessableEntity)
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/rewards/%d", feed), map[string]any{"shareable": true}, adminHeaders(), http.StatusBadRequest)
}

func TestPetHidesAndGetsSickAfterMissedChores(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.enablePets(t)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Henny", "species": "chicken"}, childHeaders(kid), http.StatusCreated)

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	twoAgo := now.AddDate(0, 0, -2).Format("2006-01-02")

	// A chore yesterday that never got done, and one today.
	resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{"title": "Feed the hens", "category": "required", "points_value": 5}, adminHeaders(), http.StatusCreated)
	var chore map[string]any
	decodeBody(t, resp, &chore)
	choreID := int(chore["id"].(float64))
	env.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/schedules", choreID),
		map[string]any{"assigned_to": kid, "specific_date": yesterday}, adminHeaders(), http.StatusCreated)
	today := env.createScheduledChore(t, "Make bed", kid, nil)

	// Pretend the pet was adopted two days ago.
	if _, err := env.db.Exec(`UPDATE pets SET last_day_checked = ?`, twoAgo); err != nil {
		t.Fatal(err)
	}
	checker := webhook.NewPetChecker(env.store)
	checker.CheckAt(context.Background(), now)
	checker.CheckAt(context.Background(), now) // idempotent

	p := env.myPet(t, kid)
	if p["health"].(float64) != 70 || p["hiding"] != true || p["mood"] != "hiding" {
		t.Fatalf("after a missed day: %v", p)
	}

	// Hiding: it won't come out to play, and no points are taken.
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": kid, "amount": 50, "note": "seed"}, adminHeaders(), http.StatusNoContent)
	play := env.petRewardID(t, kid, "play")
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", play), nil, childHeaders(kid), http.StatusUnprocessableEntity)
	if env.balance(t, kid) != 50 {
		t.Fatal("refused play still cost points")
	}

	// Doing today's chores brings it back out.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", today), map[string]any{}, childHeaders(kid), http.StatusCreated)
	if p := env.myPet(t, kid); p["hiding"] != false {
		t.Fatalf("still hiding after today's chores: %v", p)
	}
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", play), nil, childHeaders(kid), http.StatusCreated)
}

func TestPetWaitsWhilePaused(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.enablePets(t)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Woolly", "species": "sheep"}, childHeaders(kid), http.StatusCreated)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{"title": "Sweep", "category": "core"}, adminHeaders(), http.StatusCreated)
	var chore map[string]any
	decodeBody(t, resp, &chore)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/schedules", int(chore["id"].(float64))),
		map[string]any{"assigned_to": kid, "specific_date": yesterday}, adminHeaders(), http.StatusCreated)
	env.db.Exec(`UPDATE pets SET last_day_checked = ?`, time.Now().AddDate(0, 0, -2).Format("2006-01-02"))
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/users/%d/pause", kid), map[string]any{}, adminHeaders(), http.StatusOK)

	webhook.NewPetChecker(env.store).CheckAt(context.Background(), time.Now())
	if p := env.myPet(t, kid); p["health"].(float64) != 80 || p["fullness"].(float64) != 70 {
		t.Fatalf("paused pet changed: %v", p)
	}
}

func TestPetNewAnimalRestyleAndEmojiAccessory(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.enablePets(t)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Biscuit", "species": "dog", "color": "purple"}, childHeaders(kid), http.StatusBadRequest)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Biscuit", "species": "dog", "color": "chocolate"}, childHeaders(kid), http.StatusCreated)
	env.db.Exec(`UPDATE pets SET health = 30`)
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": kid, "amount": 500, "note": "seed"}, adminHeaders(), http.StatusNoContent)

	// New animal: the pick is required, and stats carry over.
	swap := env.petRewardID(t, kid, "new_animal")
	before := env.balance(t, kid)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", swap), nil, childHeaders(kid), http.StatusUnprocessableEntity)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", swap), map[string]any{"pet": map[string]any{"species": "dog", "color": "chocolate"}}, childHeaders(kid), http.StatusUnprocessableEntity)
	if env.balance(t, kid) != before {
		t.Fatal("refused swap cost points")
	}
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", swap), map[string]any{"pet": map[string]any{"species": "horse"}}, childHeaders(kid), http.StatusCreated)
	p := env.myPet(t, kid)
	if p["species"] != "horse" || p["color"] != "chestnut" || p["health"].(float64) != 30 || p["name"] != "Biscuit" {
		t.Fatalf("after swap: %v", p)
	}

	// Restyle: a coat of the current animal.
	restyle := env.petRewardID(t, kid, "restyle")
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", restyle), map[string]any{"pet": map[string]any{"color": "ginger"}}, childHeaders(kid), http.StatusUnprocessableEntity)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", restyle), map[string]any{"pet": map[string]any{"color": "palomino"}}, childHeaders(kid), http.StatusCreated)
	if p := env.myPet(t, kid); p["color"] != "palomino" || p["species"] != "horse" {
		t.Fatalf("after restyle: %v", p)
	}

	// Custom emoji accessories.
	env.expectStatus(t, "POST", "/api/rewards", map[string]any{"name": "Bad", "cost": 5, "pet_action": "accessory", "pet_item": "e:tail:🎓"}, adminHeaders(), http.StatusBadRequest)
	resp := env.expectStatus(t, "POST", "/api/rewards", map[string]any{"name": "Grad cap", "cost": 5, "pet_action": "accessory", "pet_item": "e:head:🎓"}, adminHeaders(), http.StatusCreated)
	var rw map[string]any
	decodeBody(t, resp, &rw)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", int(rw["id"].(float64))), nil, childHeaders(kid), http.StatusCreated)
	if p := env.myPet(t, kid); p["accessory"] != "e:head:🎓" {
		t.Fatalf("emoji accessory: %v", p)
	}
}

func TestPetDesignAccessory(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.enablePets(t)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Biscuit", "species": "dog"}, childHeaders(kid), http.StatusCreated)
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": kid, "amount": 100, "note": "seed"}, adminHeaders(), http.StatusNoContent)
	design := env.petRewardID(t, kid, "design")

	// A 600x300 picture: shrunk to fit 256.
	img := image.NewNRGBA(image.Rect(0, 0, 600, 300))
	for x := 100; x < 500; x++ {
		img.SetNRGBA(x, 150, color.NRGBA{R: 255, A: 255})
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	before := env.balance(t, kid)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", design), map[string]any{"pet": map[string]any{"slot": "tail", "image": dataURL}}, childHeaders(kid), http.StatusUnprocessableEntity)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", design), map[string]any{"pet": map[string]any{"slot": "head", "image": "data:image/png;base64,bm90IGFuIGltYWdl"}}, childHeaders(kid), http.StatusUnprocessableEntity)
	if env.balance(t, kid) != before {
		t.Fatal("refused design cost points")
	}
	env.expectStatus(t, "POST", fmt.Sprintf("/api/rewards/%d/redeem", design), map[string]any{"pet": map[string]any{"slot": "head", "image": dataURL}}, childHeaders(kid), http.StatusCreated)
	p := env.myPet(t, kid)
	acc, _ := p["accessory"].(string)
	if !strings.HasPrefix(acc, "img:head:") {
		t.Fatalf("design accessory: %v", p)
	}
	path := "data/uploads/pets/" + strings.TrimPrefix(acc, "img:head:") + ".png"
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("saved picture: %v", err)
	}
	defer os.Remove(path)
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil || cfg.Width != 256 || cfg.Height != 128 {
		t.Fatalf("picture size %dx%d (%v)", cfg.Width, cfg.Height, err)
	}
}
