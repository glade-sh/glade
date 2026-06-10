package debuglog

import "github.com/glade-sh/glade/internal/apexlog"

type AnnotatedLog struct {
	Log     apexlog.Log      `json:"log"`
	Entries []AnnotatedEntry `json:"entries"`
}

type AnnotatedEntry struct {
	Entry      apexlog.Entry     `json:"entry"`
	Best       SourceCandidate   `json:"best,omitempty"`
	Candidates []SourceCandidate `json:"candidates,omitempty"`
}

type SourceCandidate struct {
	File       string  `json:"file,omitempty"`
	Line       int     `json:"line,omitempty"`
	Symbol     string  `json:"symbol,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	Confidence float64 `json:"confidence"`
}
