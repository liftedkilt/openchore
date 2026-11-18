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

// spritePNG makes a w x h PNG data URL on a see-through background (unless
// opaque), with a filled square in the middle of each of the first `drawn`
// frames (frames are h x h, side by side).
func spritePNG(t *testing.T, w, h int, opaque bool, drawn int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fx := x % h
			switch {
			case x/h < drawn && fx > h/4 && fx < 3*h/4 && y > h/4 && y < 3*h/4:
				img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 120, B: 40, A: 255})
			case opaque:
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestPetOwnPicture(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kid := env.createChild(t, "Kid")
	env.enablePets(t)
	env.expectStatus(t, "POST", "/api/me/pet", map[string]any{"name": "Quackers", "species": "chicken"}, childHeaders(kid), http.StatusCreated)
	env.expectStatus(t, "POST", "/api/points/adjust", map[string]any{"user_id": kid, "amount": 500, "note": "seed"}, adminHeaders(), http.StatusNoContent)
	reward := env.petRewardID(t, kid, "picture")
	path := fmt.Sprintf("/api/rewards/%d/redeem", reward)
	redeem := func(image string, want int) {
		t.Helper()
		env.expectStatus(t, "POST", path, map[string]any{"pet": map[string]any{"image": image}}, childHeaders(kid), want).Body.Close()
	}

	// Refused before any points move: nothing sent, not a PNG, no
	// see-through background, too small.
	before := env.balance(t, kid)
	env.expectStatus(t, "POST", path, map[string]any{}, childHeaders(kid), http.StatusUnprocessableEntity)
	redeem("data:image/jpeg;base64,AAAA", http.StatusUnprocessableEntity)
	redeem("data:image/png;base64,bm90IGFuIGltYWdl", http.StatusUnprocessableEntity)
	redeem(spritePNG(t, 64, 64, true, 1), http.StatusUnprocessableEntity)
	redeem(spritePNG(t, 4, 4, false, 1), http.StatusUnprocessableEntity)
	redeem(spritePNG(t, 192, 64, false, 0), http.StatusUnprocessableEntity) // a strip with nothing in the standing frame
	// A strip with a gap (pictures 1 to 3 and 6, nothing in 4 and 5).
	gap := image.NewNRGBA(image.Rect(0, 0, 6*16, 16))
	for _, f := range []int{0, 1, 2, 5} {
		gap.SetNRGBA(f*16+8, 8, color.NRGBA{R: 200, A: 255})
	}
	var gapBuf bytes.Buffer
	png.Encode(&gapBuf, gap)
	redeem("data:image/png;base64,"+base64.StdEncoding.EncodeToString(gapBuf.Bytes()), http.StatusUnprocessableEntity)
	if env.balance(t, kid) != before {
		t.Fatal("a refused picture cost points")
	}

	// A good one is stored, kept at its own size, and shown on the farm.
	redeem(spritePNG(t, 64, 64, false, 1), http.StatusCreated)
	first, _ := env.myPet(t, kid)["sprite"].(string)
	if first == "" {
		t.Fatalf("no picture on the pet: %v", env.myPet(t, kid))
	}
	file := func(id string) string { return "data/uploads/pets/" + id + ".png" }
	t.Cleanup(func() { os.Remove(file(first)) })
	f, err := os.Open(file(first))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width != 64 || cfg.Height != 64 {
		t.Fatalf("stored picture: %v %+v", err, cfg)
	}
	resp := env.expectStatus(t, "GET", "/api/pets", nil, nil, http.StatusOK)
	var farm map[string]any
	decodeBody(t, resp, &farm)
	if p := farm["pets"].([]any)[0].(map[string]any); p["sprite"] != first || p["species"] != "chicken" || p["sprite_frames"] != 1.0 {
		t.Fatalf("farm: %v", p)
	}

	// A big single picture is scaled down; the one it replaces is deleted.
	redeem(spritePNG(t, 640, 480, false, 1), http.StatusCreated)
	second := env.myPet(t, kid)["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(second)) })
	if _, err := os.Stat(file(first)); !os.IsNotExist(err) {
		t.Fatal("the replaced picture was left on disk")
	}
	f, _ = os.Open(file(second))
	cfg, err = png.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width != 128 || cfg.Height != 96 {
		t.Fatalf("scaled picture: %v %+v", err, cfg)
	}

	// A strip of six square frames is kept as six frames, scaled by its
	// height. Blank frames at the end are dropped.
	redeem(spritePNG(t, 6*320, 320, false, 6), http.StatusCreated)
	strip := env.myPet(t, kid)
	stripID := strip["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(stripID)) })
	f, _ = os.Open(file(stripID))
	cfg, err = png.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width != 6*128 || cfg.Height != 128 || strip["sprite_frames"] != 6.0 {
		t.Fatalf("strip: %v %+v frames=%v", err, cfg, strip["sprite_frames"])
	}
	redeem(spritePNG(t, 6*64, 64, false, 3), http.StatusCreated)
	short := env.myPet(t, kid)
	second = short["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(second)) })
	f, _ = os.Open(file(second))
	cfg, err = png.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width != 3*64 || cfg.Height != 64 || short["sprite_frames"] != 3.0 {
		t.Fatalf("short strip: %v %+v frames=%v", err, cfg, short["sprite_frames"])
	}

	// Going back to the drawn animal is free; setting a picture without a reward isn't allowed.
	env.expectStatus(t, "PATCH", "/api/me/pet", map[string]any{"sprite": "abcdef0123456789abcdef01"}, childHeaders(kid), http.StatusUnprocessableEntity)
	env.expectStatus(t, "PATCH", "/api/me/pet", map[string]any{"sprite": ""}, childHeaders(kid), http.StatusOK)
	if p := env.myPet(t, kid); p["sprite"] != nil {
		t.Fatalf("picture not cleared: %v", p)
	}
	if _, err := os.Stat(file(second)); !os.IsNotExist(err) {
		t.Fatal("a cleared picture was left on disk")
	}

	// A grown-up can take a picture away; a kid can't take a sibling's.
	redeem(spritePNG(t, 32, 32, false, 1), http.StatusCreated)
	third := env.myPet(t, kid)["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(third)) })
	sib := env.createChild(t, "Sib")
	rm := fmt.Sprintf("/api/admin/users/%d/pet/picture", kid)
	env.expectStatus(t, "DELETE", rm, nil, childHeaders(sib), http.StatusForbidden)
	env.expectStatus(t, "DELETE", rm, nil, adminHeaders(), http.StatusNoContent)
	if p := env.myPet(t, kid); p["sprite"] != nil || p["name"] != "Quackers" {
		t.Fatalf("after a grown-up removed the picture: %v", p)
	}

	// Removing the animal removes its picture file too.
	redeem(spritePNG(t, 32, 32, false, 1), http.StatusCreated)
	last := env.myPet(t, kid)["sprite"].(string)
	t.Cleanup(func() { os.Remove(file(last)) })
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/admin/users/%d/pet", kid), nil, adminHeaders(), http.StatusNoContent)
	if _, err := os.Stat(file(last)); !os.IsNotExist(err) {
		t.Fatal("a removed animal's picture was left on disk")
	}
}
