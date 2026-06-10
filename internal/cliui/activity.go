package cliui

import "strings"

type ActivityFeed struct {
	limit int
	lines []string
}

func NewActivityFeed(limit int) *ActivityFeed {
	if limit <= 0 {
		limit = 5
	}
	return &ActivityFeed{limit: limit}
}

func (f *ActivityFeed) Add(ev Event) {
	if f == nil {
		return
	}
	line := strings.TrimSpace(ev.Label)
	if ev.Detail != "" {
		line += " - " + strings.TrimSpace(ev.Detail)
	}
	if line == "" {
		return
	}
	f.lines = append(f.lines, line)
	if len(f.lines) > f.limit {
		f.lines = append([]string(nil), f.lines[len(f.lines)-f.limit:]...)
	}
}

func (f *ActivityFeed) Lines() []string {
	if f == nil {
		return nil
	}
	return append([]string(nil), f.lines...)
}
