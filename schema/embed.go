// Package jsonschema embeds the Petri net model JSON Schema.
package jsonschema

import (
	_ "embed"
)

// SchemaJSON contains the JSON Schema for Petri net models.
//
//go:embed petri-model.schema.json
var SchemaJSON []byte
