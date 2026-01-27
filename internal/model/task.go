package model

import (
	"fmt"
	"strings"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
)

func ParseTaskStatus(s string) (TaskStatus, error) {
	switch strings.ToLower(s) {
	case "pending":
		return StatusPending, nil
	case "in_progress":
		return StatusInProgress, nil
	case "completed":
		return StatusCompleted, nil
	}
	return "", fmt.Errorf("invalid status: %s", s)
}

func (s TaskStatus) String() string {
	return string(s)
}

type Task struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Status       TaskStatus `json:"status"`
	Tags         []string   `json:"tags,omitempty"`
	Archived     bool       `json:"archived"`
	CommitHash   *string    `json:"commit_hash,omitempty"`
	CreatedAt    int64      `json:"created_at"`
	UpdatedAt    int64      `json:"updated_at"`
	Dependencies []int64    `json:"dependencies,omitempty"`
}

type TaskWithDeps struct {
	Task
	Dependencies []int64 `json:"dependencies"`
}
