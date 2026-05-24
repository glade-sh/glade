package apextest

import "github.com/glade-sh/glade/internal/storage"

type IsolationMode string

const (
	IsolationJournaled IsolationMode = "journaled"
	IsolationCloned    IsolationMode = "cloned"
)

type IsolationJournal = storage.IsolationJournal
