// Package services embeds service Petri net models.
package services

import (
	"embed"
	"path"
	"strings"
)

//go:embed *.json
var FS embed.FS

// List returns the names of all embedded service models (without .json extension).
func List() []string {
	entries, err := FS.ReadDir(".")
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			name := strings.TrimSuffix(entry.Name(), ".json")
			names = append(names, name)
		}
	}
	return names
}

// Get returns the content of a service model by name.
// Name should be without the .json extension.
func Get(name string) ([]byte, error) {
	return FS.ReadFile(name + ".json")
}

// GetByFilename returns the content of a service by filename.
func GetByFilename(filename string) ([]byte, error) {
	return FS.ReadFile(path.Base(filename))
}
