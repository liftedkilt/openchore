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
			{ID: "1", Name: strings.Repeat("n", 500), Days: []model.ScoreDay{
				{Date: "2026-11-16", Assigned: 3, Done: 9, Bonus: -4, AssignedPoints: 10, DonePoints: 99},
			}, Pet: &model.FriendPet{Name: "Rex", Species: "dragon"}},
			{ID: "2", Name: "Mia", Pet: &model.FriendPet{
				Name: "Clover", Species: "horse", Color: "<script>", Accessory: "img:head:0123456789abcdef0123",
				Mood: "furious", Health: 900, Fullness: -5, Joy: 50,
			}},
			{ID: "3", Name: "Theo", Pet: &model.FriendPet{Name: "Socks", Species: "cat", Color: "ginger", Accessory: "e:head:🎓", Mood: "sick"}},
		},
	}
	sanitize(&s)
	if len([]rune(s.Family)) != maxNameLen || len([]rune(s.Members[0].Name)) != maxNameLen {
		t.Errorf("names not clipped: %d %d", len(s.Family), len(s.Members[0].Name))
	}
	if d := s.Members[0].Days[0]; d.Done != 3 || d.Bonus != 0 || d.DonePoints != 10 {
		t.Errorf("day not bounded: %+v", d)
	}
	if s.Members[0].Pet != nil {
		t.Errorf("unknown animal kept: %+v", s.Members[0].Pet)
	}
	if p := s.Members[1].Pet; p == nil || p.Color != "" || p.Accessory != "" || p.Mood != "content" || p.Health != 100 || p.Fullness != 0 {
		t.Errorf("pet not cleaned: %+v", p)
	}
	if p := s.Members[2].Pet; p == nil || p.Color != "ginger" || p.Accessory != "e:head:🎓" || p.Mood != "sick" {
		t.Errorf("valid pet changed: %+v", p)
	}
}
