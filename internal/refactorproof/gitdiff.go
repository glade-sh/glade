package refactorproof

import "github.com/glade-sh/glade/internal/watch"

type ChangedFile struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Operation string `json:"operation"`
	Symbol    string `json:"symbol,omitempty"`
}

func ChangedFiles(root, since string) ([]ChangedFile, error) {
	changes, err := watch.GitChangesSince(root, since)
	if err != nil {
		return nil, err
	}
	out := make([]ChangedFile, 0, len(changes))
	for _, change := range changes {
		out = append(out, changedFileFromWatch(change))
	}
	return out, nil
}

func changedFileFromWatch(change watch.Change) ChangedFile {
	symbol := change.Name
	if symbol == "" {
		symbol = change.ObjectName
	}
	return ChangedFile{
		Path:      change.Path,
		Kind:      string(change.Kind),
		Operation: string(change.Op),
		Symbol:    symbol,
	}
}
