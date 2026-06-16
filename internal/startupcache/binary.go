package startupcache

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

const (
	stateGobFile           = "startup.gob"
	stateHeaderFile        = "startup.meta.json"
	testCacheFormatVersion = 1
	payloadFilePrefix      = "startup.payload."
	payloadFileSuffix      = ".gob"
)

var errMalformedTestCacheHeader = errors.New("malformed test cache header")

type testCacheHeader struct {
	FormatVersion int      `json:"formatVersion"`
	Version       int      `json:"version"`
	ProjectRoot   string   `json:"projectRoot"`
	BuiltAt       string   `json:"builtAt"`
	Manifest      Manifest `json:"manifest"`
	PayloadFile   string   `json:"payloadFile"`
	PayloadSHA256 string   `json:"payloadSha256"`
	PayloadSize   int64    `json:"payloadSize"`
}

type testCachePayload struct {
	Org     storage.OrgState `json:"org"`
	Runtime CompiledRuntime  `json:"runtime"`
}

func Clear(projectRoot, subdir string) error {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	root = filepath.Clean(root)
	cacheDir := filepath.Join(root, filepath.FromSlash(subdir))
	removeFile := func(name string) error {
		err := os.Remove(filepath.Join(cacheDir, name))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if subdir != SubdirTest {
		return removeFile(stateFile)
	}
	if err := removeFile(stateHeaderFile); err != nil {
		return err
	}
	if err := removeFile(stateGobFile); err != nil {
		return err
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, payloadFilePrefix) && strings.HasSuffix(name, payloadFileSuffix) {
			if err := removeFile(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func readGob(projectRoot, subdir string) (*Entry, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	if subdir == SubdirTest {
		return readSplitTestCache(root, subdir)
	}
	return readLegacyGob(root, subdir)
}

func readSplitTestCache(projectRoot, subdir string) (*Entry, error) {
	cacheDir := filepath.Join(projectRoot, filepath.FromSlash(subdir))
	header, err := readTestCacheHeader(filepath.Join(cacheDir, stateHeaderFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return readLegacyGob(projectRoot, subdir)
		}
		if errors.Is(err, errMalformedTestCacheHeader) {
			return nil, nil
		}
		return nil, err
	}
	if header == nil {
		return readLegacyGob(projectRoot, subdir)
	}
	if header.FormatVersion != testCacheFormatVersion {
		return nil, nil
	}
	if !freshManifest(header.Version, header.ProjectRoot, header.Manifest, projectRoot, Version) {
		return nil, nil
	}
	if !validPayloadFileName(header.PayloadFile, header.PayloadSHA256) {
		return nil, nil
	}
	payloadPath := filepath.Join(cacheDir, header.PayloadFile)
	payload, err := readTestCachePayload(payloadPath, header.PayloadSHA256, header.PayloadSize)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, nil
	}
	return &Entry{
		Version:     header.Version,
		ProjectRoot: header.ProjectRoot,
		BuiltAt:     header.BuiltAt,
		Manifest:    header.Manifest,
		Org:         payload.Org,
		Runtime:     payload.Runtime,
	}, nil
}

func readTestCacheHeader(path string) (*testCacheHeader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	var header testCacheHeader
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("%w: %v", errMalformedTestCacheHeader, err)
	}
	return &header, nil
}

func readTestCachePayload(path string, wantHash string, wantSize int64) (testCachePayload, error) {
	if strings.TrimSpace(wantHash) == "" || wantSize < 0 {
		return testCachePayload{}, fmt.Errorf("invalid payload header")
	}
	file, err := os.Open(path)
	if err != nil {
		return testCachePayload{}, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return testCachePayload{}, err
	}
	if stat.Size() != wantSize {
		return testCachePayload{}, fmt.Errorf("payload size mismatch")
	}
	hasher := sha256.New()
	reader := &hashingReader{reader: file, hash: hasher}
	var payload testCachePayload
	if err := gob.NewDecoder(reader).Decode(&payload); err != nil {
		return testCachePayload{}, err
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return testCachePayload{}, err
	}
	if reader.n != wantSize {
		return testCachePayload{}, fmt.Errorf("payload size mismatch")
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); got != wantHash {
		return testCachePayload{}, fmt.Errorf("payload hash mismatch")
	}
	return payload, nil
}

func readLegacyGob(projectRoot, subdir string) (*Entry, error) {
	path := filepath.Join(projectRoot, filepath.FromSlash(subdir), stateGobFile)
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	var entry Entry
	dec := gob.NewDecoder(file)
	if err := dec.Decode(&entry); err != nil {
		return nil, nil
	}
	return &entry, nil
}

func writeGob(entry *Entry, subdir string) error {
	if entry == nil || entry.ProjectRoot == "" {
		return errors.New("startup cache entry requires project root")
	}
	if subdir == SubdirTest {
		return writeSplitTestCache(entry, subdir)
	}
	return writeLegacyGob(entry, subdir)
}

func writeSplitTestCache(entry *Entry, subdir string) error {
	cacheDir := filepath.Join(entry.ProjectRoot, filepath.FromSlash(subdir))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	payload := testCachePayload{Org: entry.Org, Runtime: entry.Runtime}
	tmp, err := os.CreateTemp(cacheDir, "startup-payload-*.gob")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	hasher := sha256.New()
	counting := &countingWriter{writer: io.MultiWriter(tmp, hasher)}
	writeErr := func() error {
		defer os.Remove(tmpPath)
		if err := gob.NewEncoder(counting).Encode(payload); err != nil {
			_ = tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		sum := hex.EncodeToString(hasher.Sum(nil))
		payloadFile := payloadFilePrefix + sum + payloadFileSuffix
		payloadPath := filepath.Join(cacheDir, payloadFile)
		if err := os.Rename(tmpPath, payloadPath); err != nil {
			return err
		}
		header := testCacheHeader{
			FormatVersion: testCacheFormatVersion,
			Version:       entry.Version,
			ProjectRoot:   entry.ProjectRoot,
			BuiltAt:       entry.BuiltAt,
			Manifest:      entry.Manifest,
			PayloadFile:   payloadFile,
			PayloadSHA256: sum,
			PayloadSize:   counting.n,
		}
		if err := writeTestCacheHeader(cacheDir, header); err != nil {
			return err
		}
		return pruneInactiveTestPayloads(cacheDir, payloadFile)
	}()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return writeErr
	}
	return nil
}

func writeTestCacheHeader(cacheDir string, header testCacheHeader) error {
	tmp, err := os.CreateTemp(cacheDir, "startup-header-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	writeErr := func() error {
		defer os.Remove(tmpPath)
		enc := json.NewEncoder(tmp)
		enc.SetIndent("", "  ")
		if err := enc.Encode(header); err != nil {
			_ = tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		return os.Rename(tmpPath, filepath.Join(cacheDir, stateHeaderFile))
	}()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return writeErr
	}
	return nil
}

func pruneInactiveTestPayloads(cacheDir, activePayloadFile string) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == activePayloadFile {
			continue
		}
		if strings.HasPrefix(name, payloadFilePrefix) && strings.HasSuffix(name, payloadFileSuffix) {
			_ = os.Remove(filepath.Join(cacheDir, name))
		}
	}
	return nil
}

func writeLegacyGob(entry *Entry, subdir string) error {
	cacheDir := filepath.Join(entry.ProjectRoot, filepath.FromSlash(subdir))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, stateGobFile)
	tmp, err := os.CreateTemp(cacheDir, "startup-*.gob")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	writeErr := func() error {
		defer os.Remove(tmpPath)
		enc := gob.NewEncoder(tmp)
		if err := enc.Encode(entry); err != nil {
			_ = tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		return os.Rename(tmpPath, path)
	}()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return writeErr
	}
	return nil
}

func validPayloadFileName(name, wantHash string) bool {
	if filepath.Base(name) != name || strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return false
	}
	if len(wantHash) != sha256HexLength || !isLowerHex(wantHash) {
		return false
	}
	return name == payloadFilePrefix+wantHash+payloadFileSuffix
}

const sha256HexLength = 64

func isLowerHex(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

type countingWriter struct {
	writer io.Writer
	n      int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.n += int64(n)
	return n, err
}

type hashingReader struct {
	reader io.Reader
	hash   hash.Hash
	n      int64
}

func (r *hashingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		_, _ = r.hash.Write(p[:n])
		r.n += int64(n)
	}
	return n, err
}
