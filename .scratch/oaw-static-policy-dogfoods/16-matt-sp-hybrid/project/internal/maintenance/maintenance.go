package maintenance

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type maintenanceWindow struct {
	raw   string
	start int
	end   int
}

// Evaluate validates a same-day Maintenance Plan and returns its stable summary.
func Evaluate(inputs []string) (string, error) {
	windows := make([]maintenanceWindow, 0, len(inputs))
	for _, input := range inputs {
		window, err := parseWindow(input)
		if err != nil {
			return "", err
		}
		windows = append(windows, window)
	}
	sort.Slice(windows, func(i, j int) bool {
		if windows[i].start == windows[j].start {
			return windows[i].end < windows[j].end
		}
		return windows[i].start < windows[j].start
	})
	for index := 1; index < len(windows); index++ {
		previous, current := windows[index-1], windows[index]
		if current.start < previous.end {
			return "", fmt.Errorf("overlap: %q and %q", previous.raw, current.raw)
		}
	}
	return fmt.Sprintf("valid maintenance plan: %d windows", len(windows)), nil
}

func parseWindow(raw string) (maintenanceWindow, error) {
	parts := strings.Split(raw, "-")
	if len(parts) != 2 {
		return maintenanceWindow{}, fmt.Errorf("invalid window %q: use HH:MM-HH:MM", raw)
	}
	start, err := parseClock(parts[0])
	if err != nil {
		return maintenanceWindow{}, fmt.Errorf("invalid window %q: use HH:MM-HH:MM", raw)
	}
	end, err := parseClock(parts[1])
	if err != nil || end <= start {
		return maintenanceWindow{}, fmt.Errorf("invalid window %q: end must be after start", raw)
	}
	return maintenanceWindow{raw: raw, start: start, end: end}, nil
}

func parseClock(raw string) (int, error) {
	if len(raw) != 5 || raw[2] != ':' || !digits(raw[:2]) || !digits(raw[3:]) {
		return 0, fmt.Errorf("invalid clock")
	}
	hour, err := strconv.Atoi(raw[:2])
	if err != nil || hour < 0 || hour > 23 {
		return 0, fmt.Errorf("invalid clock")
	}
	minute, err := strconv.Atoi(raw[3:])
	if err != nil || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("invalid clock")
	}
	return hour*60 + minute, nil
}

func digits(raw string) bool {
	for _, char := range raw {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
