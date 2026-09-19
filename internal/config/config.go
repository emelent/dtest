// Package config reads dtest's TOML file, which rebinds keys and repaints
// the palette. Everything in it is optional: a missing file, a missing
// table or a missing entry all mean the built-in default stands.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the whole file. Both tables are keyed by the name of the thing
// being set: an action for Keys, a role for Colors.
type Config struct {
	Keys   map[string]Binding `toml:"keys"`
	Colors map[string]string  `toml:"colors"`
}

// Binding is the keys bound to one action. It reads either a single key,
// `quit = "q"`, or several, `down = ["j", "down"]`, since one action often
// answers to more than one key and writing a list for all of them is noise.
type Binding []string

// UnmarshalTOML accepts a string or an array of strings.
func (b *Binding) UnmarshalTOML(v any) error {
	switch v := v.(type) {
	case string:
		*b = Binding{v}
		return nil
	case []any:
		keys := make(Binding, 0, len(v))
		for _, k := range v {
			s, ok := k.(string)
			if !ok {
				return fmt.Errorf("want a key name, got %v", k)
			}
			keys = append(keys, s)
		}
		*b = keys
		return nil
	}
	return fmt.Errorf("want a key name or a list of them, got %v", v)
}

// Path is where dtest looks for its config: $DTEST_CONFIG when that is set,
// otherwise dtest/config.toml under $XDG_CONFIG_HOME or ~/.config.
func Path() string {
	if p := os.Getenv("DTEST_CONFIG"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "dtest", "config.toml")
}

// Load reads the file at path. A path that is empty or names nothing gives
// an empty config and no error: having no config file is the normal case.
func Load(path string) (Config, error) {
	var c Config
	if path == "" {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := toml.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}
