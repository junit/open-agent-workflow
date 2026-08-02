package check

import (
	"os"
	"strings"
)

const (
	beginMarker = "<!-- BEGIN OPEN AGENT WORKFLOW -->"
	endMarker   = "<!-- END OPEN AGENT WORKFLOW -->"
)

func managedBlock(path string) (string, []byte) {
	info, err := os.Stat(path)
	if err != nil {
		return "absent", nil
	}
	if !info.Mode().IsRegular() {
		return "drift", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "drift", nil
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) != 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	beginCount, endCount := 0, 0
	beginLine, endLine := -1, -1
	for index, line := range lines {
		switch line {
		case beginMarker:
			beginCount++
			if beginLine == -1 {
				beginLine = index
			}
		case endMarker:
			endCount++
			if endLine == -1 {
				endLine = index
			}
		}
	}
	if beginCount == 0 && endCount == 0 {
		return "absent", nil
	}
	if beginCount != 1 || endCount != 1 || beginLine >= endLine {
		return "drift", nil
	}
	block := strings.Join(lines[beginLine:endLine+1], "\n") + "\n"
	return "present", []byte(block)
}
