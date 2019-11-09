package search

import (
	"strings"
)

func excerpt(text string, start, end, maxChars int) string {
	origStart := start
	origEnd := end

	start -= maxChars / 2
	if start < 0 {
		start = 0
	}

	end += maxChars / 2
	if end > len(text) {
		end = len(text)
	}

	const breakChars = ".\n"

	if index := strings.IndexAny(text[start:origStart], breakChars); index != -1 {
		start += index + 1
		end += index
		if end > len(text) {
			end = len(text)
		}
	}

	if index := strings.LastIndexAny(text[origEnd:end], breakChars); index != -1 {
		end = origEnd + index + 1
		if end > len(text) {
			end = len(text)
		}
	}

	return strings.TrimSpace(text[start:end])
}
