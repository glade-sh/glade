package lsp

import (
	"crypto/sha256"
	"os"
	"sort"
	"strings"
	"time"
)

type occurrenceFile struct {
	size     int64
	modified time.Time
	digest   [sha256.Size]byte
	valid    bool
}
type occurrenceIndex struct {
	files map[string]occurrenceFile
	names map[string][]string
}

func (h *requestHandler) referenceCandidates(name string) []string {
	for i := 0; i < len(name); i++ {
		if !isIdentifierByte(name[i]) {
			return h.sourceFiles()
		}
	}
	h.generation.occurrenceOnce.Do(func() {
		idx := occurrenceIndex{files: make(map[string]occurrenceFile), names: make(map[string][]string)}
		seen := map[string]bool{}
		add := func(path string) {
			if path == "" || seen[path] {
				return
			}
			seen[path] = true
			info, err := os.Stat(path)
			if err != nil {
				idx.files[path] = occurrenceFile{}
				return
			}
			data, err := os.ReadFile(path)
			if err != nil {
				idx.files[path] = occurrenceFile{}
				return
			}
			idx.files[path] = occurrenceFile{size: info.Size(), modified: info.ModTime(), digest: sha256.Sum256(data), valid: true}
			words := map[string]bool{}
			for i := 0; i < len(data); {
				if !isIdentifierByte(data[i]) {
					i++
					continue
				}
				start := i
				for i < len(data) && isIdentifierByte(data[i]) {
					i++
				}
				words[strings.ToLower(string(data[start:i]))] = true
			}
			for word := range words {
				idx.names[word] = append(idx.names[word], path)
			}
		}
		for _, typ := range h.index.Types {
			add(typ.File)
		}
		for _, tr := range h.index.Triggers {
			add(tr.File)
		}
		h.generation.occurrences = idx
	})
	idx := h.generation.occurrences
	candidates := map[string]bool{}
	for _, p := range idx.names[strings.ToLower(name)] {
		candidates[p] = true
	}
	for p, old := range idx.files {
		info, err := os.Stat(p)
		if candidates[p] {
			continue
		}
		if err != nil || !old.valid || info.Size() != old.size || !info.ModTime().Equal(old.modified) {
			candidates[p] = true
			continue
		}
		// Metadata can be preserved by a replacement. Only omit a file when
		// its contents still match the data from which the index was built.
		data, err := os.ReadFile(p)
		if err != nil || sha256.Sum256(data) != old.digest {
			candidates[p] = true
		}
	}
	h.docMu.RLock()
	for _, doc := range h.documents {
		candidates[doc.Path] = true
	}
	h.docMu.RUnlock()
	files := make([]string, 0, len(candidates))
	for p := range candidates {
		files = append(files, p)
	}
	sort.Strings(files)
	return files
}
