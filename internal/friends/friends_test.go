package friends

import (
	"strings"
	"testing"

	"github.com/liftedkilt/openchore/internal/model"
)

// Whatever another family's server sends, only values this version
// understands and reasonable sizes are kept.
func TestSanitizeBoundsWhatAPeerCanSend(t *testing.T) {
	s := model.FriendSnapshot{
		Family: strings.Repeat("x", 500),
		Members: []model.FriendMember{
			{ID: "1", Name: strings.Repeat("n", 500), Weeks: []model.FriendWeek{
				{Start: "2026-11-02"}, {Start: "2026-11-09"},
				{Start: "2026-11-16", Assigned: 3, Done: 9, Bonus: -4, Share: 123456},
			}, Pet: &model.FriendPet{Name: "Rex", Species: "dragon"}},
			{ID: "2", Name: "Mia", Pet: &model.FriendPet{
				Name: "Clover", Species: "horse", Color: "<script>", Accessory: "img:head:0123456789abcdef0123",
				Mood: "furious",
			}},
			{ID: "3", Name: "Theo", Pet: &model.FriendPet{Name: "Socks", Species: "cat", Color: "ginger", Accessory: "e:head:🎓", Mood: "sick"}},
		},
	}
	s.Today = "not a date"
	s.Members[2].Streak = 1 << 40
	s.Members = append(s.Members,
		model.FriendMember{ID: "2", Name: "Impostor"}, // a second person using Mia's id
		model.FriendMember{ID: "", Name: "Nobody"},
		model.FriendMember{ID: "9", Name: "Odd weeks", Weeks: []model.FriendWeek{{Start: "<b>"}, {Start: "2026-11-16", Assigned: 1, Done: 1, Share: 10000}}})
	sanitize(&s)
	if len(s.Members) != 4 || s.Members[1].Name != "Mia" || s.Members[3].Name != "Odd weeks" || s.Today != "" {
		t.Fatalf("members after cleaning: %+v today=%q", s.Members, s.Today)
	}
	if s.Members[2].Streak != 9999 || len(s.Members[3].Weeks) != 1 {
		t.Errorf("streak or week starts not bounded: %d %+v", s.Members[2].Streak, s.Members[3].Weeks)
	}
	if len([]rune(s.Family)) != maxNameLen || len([]rune(s.Members[0].Name)) != maxNameLen {
		t.Errorf("names not clipped: %d %d", len(s.Family), len(s.Members[0].Name))
	}
	if ws := s.Members[0].Weeks; len(ws) != 2 || ws[1].Done != 3 || ws[1].Bonus != 0 || ws[1].Share != 10000 {
		t.Errorf("weeks not bounded: %+v", ws)
	}
	if s.Members[0].Pet != nil {
		t.Errorf("unknown animal kept: %+v", s.Members[0].Pet)
	}
	if p := s.Members[1].Pet; p == nil || p.Color != "" || p.Accessory != "" || p.Mood != "content" {
		t.Errorf("pet not cleaned: %+v", p)
	}
	if p := s.Members[2].Pet; p == nil || p.Color != "ginger" || p.Accessory != "e:head:🎓" || p.Mood != "sick" {
		t.Errorf("valid pet changed: %+v", p)
	}
}

// Sealed data only opens for the other side of the link: a hub can't hand a
// family its own snapshot back as if it came from their friends.
func TestSealIsBoundToTheSender(t *testing.T) {
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	a := &model.FriendLink{HubLink: "L", Secret: key, Side: "a"}
	b := &model.FriendLink{HubLink: "L", Secret: key, Side: "b"}
	sealed, err := seal(a, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := open(b, sealed); err != nil || string(got) != "hello" {
		t.Fatalf("the other side can't read it: %v %q", err, got)
	}
	if _, err := open(a, sealed); err == nil {
		t.Fatal("a side could open its own sealed data as if it were the other family's")
	}
	other := &model.FriendLink{HubLink: "M", Secret: key, Side: "b"}
	if _, err := open(other, sealed); err == nil {
		t.Fatal("sealed data opened on a different link")
	}
}
