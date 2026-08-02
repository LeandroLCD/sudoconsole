package config

import "github.com/LeandroLCD/sudoconsole/internal/domain"

// DefaultFileSchema returns the TOML representation of
// domain.DefaultConfig. It is exported so callers building a config
// from scratch (e.g. `sudoconsole config init`) can mirror the same
// projection used by Save.
func DefaultFileSchema() fileSchema {
	return newFileSchemaFromConfig(domain.DefaultConfig())
}
