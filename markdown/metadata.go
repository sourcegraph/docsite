package markdown

import (
	"bytes"

	"gopkg.in/yaml.v2"
)

// Metadata is document metadata in the "front matter" of a Markdown document.
type Metadata struct {
	Title                       string `yaml:"title"`
	IgnoreDisconnectedPageCheck bool   `yaml:"ignoreDisconnectedPageCheck"`
}

func parseMetadata(input []byte) (meta Metadata, markdown []byte, err error) {
	// YAML metadata delimiter is "---" on its own line.
	const (
		startMarker = "---\n"
		endMarker   = "\n---\n"
	)
	if !bytes.HasPrefix(input, []byte(startMarker)) {
		return meta, input, nil // no metadata (because no starting delimiter)
	}
	end := bytes.Index(input[len(startMarker):], []byte(endMarker))
	if end == -1 {
		return meta, input, nil // no metadata (because no ending delimiter)
	}

	err = yaml.Unmarshal(input[:len(startMarker)+end], &meta)
	markdown = input[len(startMarker)+end+len(endMarker):]
	return meta, markdown, err
}
