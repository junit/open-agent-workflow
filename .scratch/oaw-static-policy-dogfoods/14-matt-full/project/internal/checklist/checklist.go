// Package checklist models the completion state of Markdown checklist Items.
package checklist

import "strings"

// Summary is the observable count of a Checklist's Items.
type Summary struct {
	Complete int
	Total    int
}

// Summarize counts Markdown task-list markers and ignores surrounding context.
func Summarize(markdown string) Summary {
	var result Summary
	for _, rawLine := range strings.Split(markdown, "\n") {
		line := strings.TrimLeft(rawLine, " \t")
		if len(line) < 5 || line[0:3] != "- [" || line[4] != ']' {
			continue
		}
		if line[3] != ' ' && line[3] != 'x' && line[3] != 'X' {
			continue
		}
		result.Total++
		if line[3] == 'x' || line[3] == 'X' {
			result.Complete++
		}
	}
	return result
}
