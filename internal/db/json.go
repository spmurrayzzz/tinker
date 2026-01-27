package db

import (
	"encoding/json"
	"fmt"
)

func TagsToJSON(tags []string) (string, error) {
	if tags == nil {
		tags = []string{}
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}
	return string(b), nil
}

func TagsFromJSON(s string) ([]string, error) {
	if s == "" || s == "null" {
		return []string{}, nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(s), &tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	if tags == nil {
		return []string{}, nil
	}
	return tags, nil
}
