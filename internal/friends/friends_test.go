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
	sanitize(&s)
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
