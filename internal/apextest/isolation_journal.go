package apextest

import "github.com/open-aer/oaer/internal/storage"

type IsolationMode string

const (
	IsolationJournaled IsolationMode = "journaled"
	IsolationCloned    IsolationMode = "cloned"
)

type IsolationJournal = storage.IsolationJournal
