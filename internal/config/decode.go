package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// decode reads a parsed .shipo into a Config, rejecting unknown keys.
func decode(root map[string]any) (*Config, error) {
	config := &Config{}

	var problems []string

	for _, name := range sortedKeys(root) {
		section, isSection := root[name].(map[string]any)
		if !isSection {
			problems = append(problems, fmt.Sprintf("%s: want a section", name))

			continue
		}

		fields, isKnown := config.sections()[name]
		if !isKnown {
			problems = append(problems, "unknown section ["+name+"]")

			continue
		}

		for _, key := range sortedKeys(section) {
			field, isField := fields[key]
			if !isField {
				problems = append(problems, fmt.Sprintf("unknown key %s.%s", name, key))

				continue
			}

			fieldError := field(section[key])
			if fieldError != nil {
				problems = append(problems, fmt.Sprintf("%s.%s: %v", name, key, fieldError))
			}
		}
	}

	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}

	return config, nil
}

type setter func(any) error

func (config *Config) sections() map[string]map[string]setter {
	text := func(target *string) setter {
		return func(data any) error {
			value, decodeError := decodeText(data)
			*target = value

			return decodeError
		}
	}

	list := func(target *List) setter {
		return func(data any) error {
			value, decodeError := decodeList(data)
			*target = value

			return decodeError
		}
	}

	toggle := func(target *Switch) setter {
		return func(data any) error {
			value, decodeError := decodeSwitch(data)
			*target = value

			return decodeError
		}
	}

	return map[string]map[string]setter{
		"version": {
			"preset":  text(&config.Version.Preset),
			"source":  text(&config.Version.Source),
			"pattern": text(&config.Version.Pattern),
		},
		"bump": {
			"level":  text(&config.Bump.Level),
			"commit": text(&config.Bump.Commit),
		},
		"build": {
			"dir":         text(&config.Build.Dir),
			"command":     list(&config.Build.Command),
			"environment": list(&config.Build.Environment),
		},
		"publish": {
			"push":     toggle(&config.Publish.Push),
			"release":  toggle(&config.Publish.Release),
			"workflow": toggle(&config.Publish.Workflow),
			"command":  list(&config.Publish.Command),
		},
	}
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
