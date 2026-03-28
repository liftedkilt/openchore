package config

import "strings"

// dayNameToInt maps lowercase day abbreviations to SQLite day-of-week integers (0=Sun).
var dayNameToInt = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// intToDayName maps SQLite day-of-week integers to abbreviations.
var intToDayName = map[int]string{
	0: "sun", 1: "mon", 2: "tue", 3: "wed", 4: "thu", 5: "fri", 6: "sat",
}

// ExpandDays takes day names (or shortcuts) and returns day-of-week ints.
// Shortcuts: "daily" = all 7, "weekdays" = mon-fri, "weekends" = sat+sun.
func ExpandDays(days []string) []int {
	var result []int
	for _, d := range days {
		switch strings.ToLower(strings.TrimSpace(d)) {
		case "daily":
			return []int{0, 1, 2, 3, 4, 5, 6}
		case "weekdays":
			result = append(result, 1, 2, 3, 4, 5)
		case "weekends":
			result = append(result, 0, 6)
		default:
			if v, ok := dayNameToInt[strings.ToLower(strings.TrimSpace(d))]; ok {
				result = append(result, v)
			}
		}
	}
	return result
}

// CollapseDays takes day-of-week ints and returns the most compact name representation.
func CollapseDays(ints []int) []string {
	set := make(map[int]bool, len(ints))
	for _, d := range ints {
		set[d] = true
	}

	if len(set) == 7 {
		return []string{"daily"}
	}

	weekdays := set[1] && set[2] && set[3] && set[4] && set[5]
	weekends := set[0] && set[6]

	if weekdays && weekends {
		return []string{"daily"}
	}
	if weekdays && !set[0] && !set[6] && len(set) == 5 {
		return []string{"weekdays"}
	}
	if weekends && !set[1] && !set[2] && !set[3] && !set[4] && !set[5] && len(set) == 2 {
		return []string{"weekends"}
	}

	// Sort and return individual names
	var result []string
	for _, d := range []int{0, 1, 2, 3, 4, 5, 6} {
		if set[d] {
			result = append(result, intToDayName[d])
		}
	}
	return result
}
