package server

import "strings"

const maxDevRunEvents = 100

type DevRunEvent struct {
	Sequence     int      `json:"sequence"`
	ID           string   `json:"id"`
	Status       string   `json:"status"`
	Label        string   `json:"label,omitempty"`
	Message      string   `json:"message,omitempty"`
	ChangedFiles []string `json:"changedFiles,omitempty"`
	StartedAt    string   `json:"startedAt,omitempty"`
	FinishedAt   string   `json:"finishedAt,omitempty"`
	DurationMS   int      `json:"durationMs,omitempty"`
	Error        string   `json:"error,omitempty"`
	Reload       bool     `json:"reload,omitempty"`
}

type devRunEventsPayload struct {
	LatestSequence int           `json:"latestSequence"`
	Runs           []DevRunEvent `json:"runs"`
}

func (s *Server) RecordDevRun(event DevRunEvent) DevRunEvent {
	if s == nil {
		return event
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordDevRunLocked(event)
}

func (s *Server) recordDevRunLocked(event DevRunEvent) DevRunEvent {
	s.nextDevRunSeq++
	event.Sequence = s.nextDevRunSeq
	event.ID = strings.TrimSpace(event.ID)
	if event.ID == "" {
		event.ID = "dev-run"
	}
	event.Status = strings.TrimSpace(event.Status)
	if event.Status == "" {
		event.Status = "running"
	}
	for i := len(s.devRunEvents) - 1; i >= 0; i-- {
		previous := s.devRunEvents[i]
		if previous.ID != event.ID {
			continue
		}
		if event.Label == "" {
			event.Label = previous.Label
		}
		if event.StartedAt == "" {
			event.StartedAt = previous.StartedAt
		}
		if len(event.ChangedFiles) == 0 {
			event.ChangedFiles = append([]string(nil), previous.ChangedFiles...)
		}
		break
	}
	s.devRunEvents = append(s.devRunEvents, event)
	if len(s.devRunEvents) > maxDevRunEvents {
		s.devRunEvents = append([]DevRunEvent(nil), s.devRunEvents[len(s.devRunEvents)-maxDevRunEvents:]...)
	}
	return event
}

func (s *Server) devRunEventsSinceLocked(since int) devRunEventsPayload {
	payload := devRunEventsPayload{LatestSequence: s.nextDevRunSeq}
	for _, event := range s.devRunEvents {
		if event.Sequence > since {
			payload.Runs = append(payload.Runs, event)
		}
	}
	if payload.Runs == nil {
		payload.Runs = []DevRunEvent{}
	}
	return payload
}
