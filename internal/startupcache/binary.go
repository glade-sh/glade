package startupcache

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

const (
	stateGobFile = "startup.gob"
	// Keep the historical filename so an older JSON header is observed and
	// rejected instead of accidentally enabling legacy-gob fallback.
	stateHeaderFile        = "startup.meta.json"
	testCacheFormatVersion = 2
	payloadFilePrefix      = "startup.payload."
	payloadFileSuffix      = ".gob"
)

var errMalformedTestCacheHeader = errors.New("malformed test cache header")

type testCacheHeader struct {
	FormatVersion int      `json:"formatVersion"`
	Version       int      `json:"version"`
	ProjectRoot   string   `json:"projectRoot"`
	BuiltAt       string   `json:"builtAt"`
	PlatformABI   string   `json:"platformAbi"`
	RuntimeABI    string   `json:"runtimeAbi,omitempty"`
	RuntimeKey    string   `json:"runtimeKey,omitempty"`
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
	cacheRoot, err := openPrivateStartupCacheDir(root, subdir, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer cacheRoot.Close()
	removeFile := func(name string) error {
		err := cacheRoot.Remove(name)
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
	directory, err := cacheRoot.Open(".")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
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
	cacheRoot, err := openPrivateStartupCacheDir(projectRoot, subdir, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer cacheRoot.Close()
	header, err := readTestCacheHeader(cacheRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return readLegacyGobInRoot(cacheRoot)
		}
		if errors.Is(err, errMalformedTestCacheHeader) {
			return nil, nil
		}
		return nil, err
	}
	if header == nil {
		return readLegacyGobInRoot(cacheRoot)
	}
	if header.FormatVersion != testCacheFormatVersion {
		return nil, nil
	}
	if !freshManifest(header.Version, header.ProjectRoot, header.PlatformABI, header.Manifest, projectRoot, Version) {
		return nil, nil
	}
	if !validPayloadFileName(header.PayloadFile, header.PayloadSHA256) {
		return nil, nil
	}
	payload, err := readTestCachePayload(cacheRoot, header.PayloadFile, header.PayloadSHA256, header.PayloadSize)
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
		PlatformABI: header.PlatformABI,
		RuntimeABI:  header.RuntimeABI,
		RuntimeKey:  header.RuntimeKey,
		Manifest:    header.Manifest,
		Org:         payload.Org,
		Runtime:     payload.Runtime,
	}, nil
}

func readSplitTestCacheWithStats(projectRoot, subdir string) (*Entry, ReadStats, error) {
	var stats ReadStats
	validationStarted := time.Now()
	cacheRoot, err := openPrivateStartupCacheDir(projectRoot, subdir, false)
	if err != nil {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		if errors.Is(err, os.ErrNotExist) {
			return nil, stats, nil
		}
		return nil, stats, err
	}
	defer cacheRoot.Close()
	header, err := readTestCacheHeader(cacheRoot)
	if err != nil {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		if errors.Is(err, os.ErrNotExist) {
			decodeStarted := time.Now()
			entry, legacyErr := readLegacyGobInRoot(cacheRoot)
			stats.DecodeNS = time.Since(decodeStarted).Nanoseconds()
			return entry, stats, legacyErr
		}
		if errors.Is(err, errMalformedTestCacheHeader) {
			return nil, stats, nil
		}
		return nil, stats, err
	}
	if header == nil {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		decodeStarted := time.Now()
		entry, legacyErr := readLegacyGobInRoot(cacheRoot)
		stats.DecodeNS = time.Since(decodeStarted).Nanoseconds()
		return entry, stats, legacyErr
	}
	if header.FormatVersion != testCacheFormatVersion ||
		!freshManifest(header.Version, header.ProjectRoot, header.PlatformABI, header.Manifest, projectRoot, Version) ||
		!validPayloadFileName(header.PayloadFile, header.PayloadSHA256) || header.PayloadSize < 0 {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		return nil, stats, nil
	}
	stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
	decodeStarted := time.Now()
	payload, err := readTestCachePayload(cacheRoot, header.PayloadFile, header.PayloadSHA256, header.PayloadSize)
	stats.DecodeNS = time.Since(decodeStarted).Nanoseconds()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, stats, err
		}
		return nil, stats, nil
	}
	return &Entry{
		Version:     header.Version,
		ProjectRoot: header.ProjectRoot,
		BuiltAt:     header.BuiltAt,
		PlatformABI: header.PlatformABI,
		RuntimeABI:  header.RuntimeABI,
		RuntimeKey:  header.RuntimeKey,
		Manifest:    header.Manifest,
		Org:         payload.Org,
		Runtime:     payload.Runtime,
	}, stats, nil
}

func readFreshSplitTestRuntimeWithSourceDigests(projectRoot, subdir string, expectedVersion int, expectedRuntimeABI, expectedRuntimeKey string, digests *typesys.SourceDigestSet, readFile func(string) ([]byte, error)) (*Entry, ReadStats, error) {
	var stats ReadStats
	validationStarted := time.Now()
	cacheRoot, err := openPrivateStartupCacheDir(projectRoot, subdir, false)
	if err != nil {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		if errors.Is(err, os.ErrNotExist) {
			return nil, stats, nil
		}
		return nil, stats, err
	}
	defer cacheRoot.Close()
	header, err := readTestCacheHeader(cacheRoot)
	if err != nil {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, errMalformedTestCacheHeader) {
			return nil, stats, nil
		}
		return nil, stats, err
	}
	if header == nil || header.FormatVersion != testCacheFormatVersion ||
		!validPayloadFileName(header.PayloadFile, header.PayloadSHA256) || header.PayloadSize < 0 ||
		expectedRuntimeABI == "" || header.RuntimeABI != expectedRuntimeABI ||
		expectedRuntimeKey == "" || header.RuntimeKey != expectedRuntimeKey ||
		!freshManifestWithSourceDigests(header.Version, header.ProjectRoot, header.PlatformABI, header.Manifest, projectRoot, expectedVersion, digests, readFile) {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		return nil, stats, nil
	}
	stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
	decodeStarted := time.Now()
	payload, err := readTestCachePayload(cacheRoot, header.PayloadFile, header.PayloadSHA256, header.PayloadSize)
	stats.DecodeNS = time.Since(decodeStarted).Nanoseconds()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, stats, err
		}
		return nil, stats, nil
	}
	return &Entry{
		Version:     header.Version,
		ProjectRoot: header.ProjectRoot,
		BuiltAt:     header.BuiltAt,
		PlatformABI: header.PlatformABI,
		RuntimeABI:  header.RuntimeABI,
		RuntimeKey:  header.RuntimeKey,
		Manifest:    header.Manifest,
		Org:         payload.Org,
		Runtime:     payload.Runtime,
	}, stats, nil
}

func readFreshSplitTestRuntimeWithValidatedInput(projectRoot, subdir string, expectedVersion int, expectedRuntimeABI, expectedRuntimeKey string, input *ValidatedInput) (*Entry, ReadStats, error) {
	var stats ReadStats
	validationStarted := time.Now()
	cacheRoot, err := openPrivateStartupCacheDir(projectRoot, subdir, false)
	if err != nil {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		if errors.Is(err, os.ErrNotExist) {
			return nil, stats, nil
		}
		return nil, stats, err
	}
	defer cacheRoot.Close()
	header, err := readTestCacheHeader(cacheRoot)
	if err != nil {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, errMalformedTestCacheHeader) {
			return nil, stats, nil
		}
		return nil, stats, err
	}
	if header == nil || header.FormatVersion != testCacheFormatVersion ||
		!validPayloadFileName(header.PayloadFile, header.PayloadSHA256) || header.PayloadSize < 0 ||
		expectedRuntimeABI == "" || header.RuntimeABI != expectedRuntimeABI ||
		expectedRuntimeKey == "" || header.RuntimeKey != expectedRuntimeKey ||
		!freshManifestWithValidatedInput(header.Version, header.ProjectRoot, header.PlatformABI, header.Manifest, projectRoot, expectedVersion, input) {
		stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
		return nil, stats, nil
	}
	stats.ValidationNS = time.Since(validationStarted).Nanoseconds()
	decodeStarted := time.Now()
	payload, err := readTestCachePayload(cacheRoot, header.PayloadFile, header.PayloadSHA256, header.PayloadSize)
	stats.DecodeNS = time.Since(decodeStarted).Nanoseconds()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, stats, err
		}
		return nil, stats, nil
	}
	return &Entry{
		Version:     header.Version,
		ProjectRoot: header.ProjectRoot,
		BuiltAt:     header.BuiltAt,
		PlatformABI: header.PlatformABI,
		RuntimeABI:  header.RuntimeABI,
		RuntimeKey:  header.RuntimeKey,
		Manifest:    header.Manifest,
		Org:         payload.Org,
		Runtime:     payload.Runtime,
	}, stats, nil
}

func readTestCacheHeader(cacheRoot *os.Root) (*testCacheHeader, error) {
	file, err := cacheRoot.Open(stateHeaderFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxEncodedTestCacheHeaderBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxEncodedTestCacheHeaderBytes {
		return nil, errMalformedTestCacheHeader
	}
	header, err := unmarshalTestCacheHeader(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errMalformedTestCacheHeader, err)
	}
	return &header, nil
}

func readTestCachePayload(cacheRoot *os.Root, name, wantHash string, wantSize int64) (testCachePayload, error) {
	if strings.TrimSpace(wantHash) == "" || wantSize < 0 {
		return testCachePayload{}, fmt.Errorf("invalid payload header")
	}
	if !validPayloadFileName(name, wantHash) {
		return testCachePayload{}, fmt.Errorf("invalid payload file name")
	}
	file, err := cacheRoot.Open(name)
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
	cacheRoot, err := openPrivateStartupCacheDir(projectRoot, subdir, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer cacheRoot.Close()
	return readLegacyGobInRoot(cacheRoot)
}

func readLegacyGobInRoot(cacheRoot *os.Root) (*Entry, error) {
	file, err := cacheRoot.Open(stateGobFile)
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
	_, err := writeSplitTestCacheWithStats(entry, subdir)
	return err
}

func writeSplitTestCacheWithStats(entry *Entry, subdir string) (WriteStats, error) {
	return writeSplitTestCacheWithStatsAfterRootOpened(entry, subdir, nil)
}

func writeSplitTestCacheWithStatsAfterRootOpened(entry *Entry, subdir string, afterRootOpened func() error) (WriteStats, error) {
	var stats WriteStats
	if entry == nil || entry.ProjectRoot == "" {
		return stats, errors.New("startup cache entry requires project root")
	}
	cacheRoot, err := openPrivateTestCacheDir(entry.ProjectRoot, subdir)
	if err != nil {
		return stats, err
	}
	defer cacheRoot.Close()
	if afterRootOpened != nil {
		if err := afterRootOpened(); err != nil {
			return stats, err
		}
	}
	payload := testCachePayload{Org: entry.Org, Runtime: entry.Runtime}
	tmp, tmpName, err := createTestCacheTemp(cacheRoot, "startup-payload-", ".gob")
	if err != nil {
		return stats, err
	}
	hasher := sha256.New()
	counting := &countingWriter{writer: io.MultiWriter(tmp, hasher)}
	writeErr := func() error {
		defer cacheRoot.Remove(tmpName)
		encodeStarted := time.Now()
		encodeErr := gob.NewEncoder(counting).Encode(payload)
		stats.EncodeNS = time.Since(encodeStarted).Nanoseconds()
		if encodeErr != nil {
			_ = tmp.Close()
			return encodeErr
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		sum := hex.EncodeToString(hasher.Sum(nil))
		payloadFile := payloadFilePrefix + sum + payloadFileSuffix
		if err := activateTestPayloadInRoot(cacheRoot, tmpName, payloadFile, counting.n); err != nil {
			return err
		}
		header := testCacheHeader{
			FormatVersion: testCacheFormatVersion,
			Version:       entry.Version,
			ProjectRoot:   entry.ProjectRoot,
			BuiltAt:       entry.BuiltAt,
			PlatformABI:   entry.PlatformABI,
			RuntimeABI:    entry.RuntimeABI,
			RuntimeKey:    entry.RuntimeKey,
			Manifest:      entry.Manifest,
			PayloadFile:   payloadFile,
			PayloadSHA256: sum,
			PayloadSize:   counting.n,
		}
		if err := writeTestCacheHeaderInRoot(cacheRoot, header); err != nil {
			return err
		}
		return pruneInactiveTestPayloadsInRoot(cacheRoot, payloadFile)
	}()
	if writeErr != nil {
		_ = cacheRoot.Remove(tmpName)
		return stats, writeErr
	}
	return stats, nil
}

func createTestCacheTemp(cacheRoot *os.Root, prefix, suffix string) (*os.File, string, error) {
	var random [16]byte
	for range 100 {
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := prefix + hex.EncodeToString(random[:]) + suffix
		file, err := cacheRoot.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return file, name, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("could not allocate unique startup cache temporary file")
}

func openPrivateTestCacheDir(projectRoot, subdir string) (*os.Root, error) {
	return openPrivateStartupCacheDirAfterLstat(projectRoot, subdir, true, nil)
}

func openPrivateTestCacheDirAfterLstat(projectRoot, subdir string, afterLstat func(component string) error) (*os.Root, error) {
	return openPrivateStartupCacheDirAfterLstat(projectRoot, subdir, true, afterLstat)
}

func openPrivateStartupCacheDir(projectRoot, subdir string, create bool) (*os.Root, error) {
	return openPrivateStartupCacheDirAfterLstat(projectRoot, subdir, create, nil)
}

func openPrivateStartupCacheDirAfterLstat(projectRoot, subdir string, create bool, afterLstat func(component string) error) (*os.Root, error) {
	cleanSubdir := filepath.Clean(filepath.FromSlash(subdir))
	if filepath.IsAbs(cleanSubdir) || cleanSubdir == "." || cleanSubdir == ".." || strings.HasPrefix(cleanSubdir, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("startup cache subdirectory %q must stay within the project root", subdir)
	}
	current, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(cleanSubdir, string(os.PathSeparator)) {
		if create {
			if err := current.Mkdir(component, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
				_ = current.Close()
				return nil, err
			}
		}
		beforeOpen, err := current.Lstat(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		if beforeOpen.Mode()&os.ModeSymlink != 0 {
			_ = current.Close()
			return nil, fmt.Errorf("startup cache path component %q must not be a symlink", component)
		}
		if !beforeOpen.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("startup cache path component %q is not a directory", component)
		}
		if afterLstat != nil {
			if err := afterLstat(component); err != nil {
				_ = current.Close()
				return nil, err
			}
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		afterOpen, err := next.Stat(".")
		if err != nil {
			_ = next.Close()
			_ = current.Close()
			return nil, err
		}
		if !os.SameFile(beforeOpen, afterOpen) {
			_ = next.Close()
			_ = current.Close()
			return nil, fmt.Errorf("startup cache path component %q changed while opening", component)
		}
		_ = current.Close()
		current = next
	}
	if create && runtime.GOOS != "windows" {
		if err := current.Chmod(".", 0o700); err != nil {
			_ = current.Close()
			return nil, fmt.Errorf("restrict startup cache directory: %w", err)
		}
	}
	return current, nil
}

func activateTestPayloadInRoot(cacheRoot *os.Root, tmpName, payloadFile string, payloadSize int64) error {
	if info, err := cacheRoot.Lstat(payloadFile); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("startup cache payload %q must be a regular file", payloadFile)
		}
		if info.Size() == payloadSize && info.Mode().Perm() == 0o600 {
			return cacheRoot.Remove(tmpName)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return cacheRoot.Rename(tmpName, payloadFile)
}

func writeTestCacheHeaderInRoot(cacheRoot *os.Root, header testCacheHeader) error {
	tmp, tmpName, err := createTestCacheTemp(cacheRoot, "startup-header-", ".json")
	if err != nil {
		return err
	}
	writeErr := func() error {
		defer cacheRoot.Remove(tmpName)
		data, err := marshalTestCacheHeader(header)
		if err != nil {
			_ = tmp.Close()
			return err
		}
		if err := tmp.Chmod(0o600); err != nil {
			_ = tmp.Close()
			return err
		}
		if _, err := tmp.Write(data); err != nil {
			_ = tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		return cacheRoot.Rename(tmpName, stateHeaderFile)
	}()
	if writeErr != nil {
		_ = cacheRoot.Remove(tmpName)
		return writeErr
	}
	return nil
}

func pruneInactiveTestPayloadsInRoot(cacheRoot *os.Root, activePayloadFile string) error {
	directory, err := cacheRoot.Open(".")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == activePayloadFile {
			continue
		}
		if strings.HasPrefix(name, payloadFilePrefix) && strings.HasSuffix(name, payloadFileSuffix) {
			_ = cacheRoot.Remove(name)
		}
	}
	return nil
}

func writeLegacyGob(entry *Entry, subdir string) error {
	cacheDir := filepath.Join(entry.ProjectRoot, filepath.FromSlash(subdir))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil { // #nosec G301 -- legacy project-local cache directories retain the established traversal contract.
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
