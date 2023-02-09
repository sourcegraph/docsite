package search

import (
	"bytes"
)

func excerpt(text []byte, start, end, maxChars int) []byte {
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

	if index := bytes.IndexAny(text[start:origStart], breakChars); index != -1 {
		start += index + 1
		end += index
		if end > len(text) {
			end = len(text)
		}
	}

	if index := bytes.LastIndexAny(text[origEnd:end], breakChars); index != -1 {
		end = origEnd + index + 1
		if end > len(text) {
			end = len(text)
		}
	}

	return bytes.TrimSpace(text[start:end])
}
