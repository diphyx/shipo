package config

import (
	"errors"
	"fmt"
)

// List is a key that takes one string or a list of strings.
type List []string

// Switch is a key that takes false, true, one string, or a list of strings.
type Switch struct {
	// IsSet is true when the key appears in the file.
	IsSet bool
	// IsOff is true for false.
	IsOff bool
	// IsOn is true for true.
	IsOn bool
	// Items holds the string values.
	Items []string
}

// Enabled reports whether the key does something.
func (value Switch) Enabled() bool {
	return !value.IsOff && (value.IsOn || len(value.Items) > 0)
}

func decodeText(data any) (string, error) {
	text, isString := data.(string)
	if !isString {
		return "", fmt.Errorf("want a string, got %s", kind(data))
	}

	return text, nil
}

func decodeList(data any) (List, error) {
	items, decodeError := decodeStrings(data)
	if decodeError != nil {
		return nil, decodeError
	}

	return List(items), nil
}

func decodeSwitch(data any) (Switch, error) {
	flag, isBool := data.(bool)
	if isBool {
		return Switch{IsSet: true, IsOn: flag, IsOff: !flag}, nil
	}

	items, decodeError := decodeStrings(data)
	if decodeError != nil {
		return Switch{}, fmt.Errorf("want true, false, a string or a list of strings, got %s", kind(data))
	}

	return Switch{IsSet: true, Items: items}, nil
}

func decodeStrings(data any) ([]string, error) {
	switch typed := data.(type) {
	case string:
		{
			return []string{typed}, nil
		}
	case []any:
		{
			if len(typed) == 0 {
				return nil, errors.New("list must not be empty")
			}

			items := make([]string, 0, len(typed))

			for _, item := range typed {
				text, isString := item.(string)
				if !isString {
					return nil, fmt.Errorf("list entries must be strings, got %s", kind(item))
				}

				items = append(items, text)
			}

			return items, nil
		}
	}

	return nil, fmt.Errorf("want a string or a list of strings, got %s", kind(data))
}

func kind(data any) string {
	switch data.(type) {
	case bool:
		{
			return "a boolean"
		}
	case string:
		{
			return "a string"
		}
	case []any:
		{
			return "a list"
		}
	case map[string]any:
		{
			return "a section"
		}
	}

	return fmt.Sprintf("%T", data)
}
