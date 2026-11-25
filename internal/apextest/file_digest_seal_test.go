package apextest

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
)

// recentSealFixture writes one file, keeps it recent for every validation,
// and reports its physical reads.
func recentSealFixture(t *testing.T, content string) (path string, captured [sha256.Size]byte, stamp filestamp.Stamp, reads *int) {
	t.Helper()
	skipWithoutStamps(t)
	resetFileDigestTrust()
	oldNow, oldHook, oldStat := fileDigestClock.Load(), fileDigestReadHook.Load(), requestDigestStatHook.Load()
	t.Cleanup(func() {
		fileDigestClock.Store(oldNow)
		fileDigestReadHook.Store(oldHook)
		requestDigestStatHook.Store(oldStat)
		resetFileDigestTrust()
	})
	path = filepath.Join(t.TempDir(), "A.cls")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	stamp, ok := filestamp.Stat(path)
	if !ok {
		t.Fatal("missing stamp")
	}
	setFileDigestClockForTesting(func() time.Time { return time.Unix(0, max(stamp.Mtime, stamp.Ctime)) })
	return path, sha256.Sum256([]byte(content)), stamp, countReads(t, path)
}

// holdStamp models a coarse filesystem that reports stamp on both sides of a
// same-size write.
func holdStamp(path string, stamp filestamp.Stamp) {
	stat := func(p string) (filestamp.Stamp, bool) {
		if p == path {
			return stamp, true
		}
		return filestamp.Stat(p)
	}
	requestDigestStatHook.Store(&stat)
}

func TestSealingRequestReadsRecentFileOnce(t *testing.T) {
	path, captured, _, reads := recentSealFixture(t, "first")
	var request requestFileDigests
	request.deferRecentReads()
	for range 10 {
		if got, err := request.current(path, captured); err != nil || got != captured {
			t.Fatalf("boundary digest = %x, %v", got, err)
		}
	}
	if *reads != 0 {
		t.Fatalf("boundaries read a recent file %d times before the seal", *reads)
	}
	if err := request.seal(); err != nil {
		t.Fatal(err)
	}
	if *reads != 1 {
		t.Fatalf("seal read the file %d times, want 1", *reads)
	}
	// A publication the request finishes later reuses the sealed digest.
	if got, err := request.current(path, captured); err != nil || got != captured || *reads != 1 {
		t.Fatalf("after seal: digest %x, %v, reads %d", got, err, *reads)
	}
	if err := request.seal(); err != nil || *reads != 1 {
		t.Fatalf("second seal: %v, reads %d", err, *reads)
	}
}

func TestSealingRequestRejectsSameStampEditAfterAnyBoundary(t *testing.T) {
	for _, boundariesBefore := range []int{0, 1, 5} {
		path, captured, stamp, reads := recentSealFixture(t, "first")
		holdStamp(path, stamp)
		var request requestFileDigests
		request.deferRecentReads()
		for range boundariesBefore {
			if _, err := request.current(path, captured); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(path, []byte("other"), 0o600); err != nil {
			t.Fatal(err)
		}
		// The stamp cannot show the write, so no boundary can either.
		if got, err := request.current(path, captured); err != nil || got != captured {
			t.Fatalf("boundary after the write = %x, %v", got, err)
		}
		err := request.seal()
		var mismatch *SourceSnapshotMismatchError
		if !errors.As(err, &mismatch) || mismatch.File != path {
			t.Fatalf("%d boundaries before the write: seal = %v", boundariesBefore, err)
		}
		if *reads != 1 {
			t.Fatalf("%d boundaries before the write: %d reads, want 1", boundariesBefore, *reads)
		}
	}
}

func TestSealingRequestReadsAtTheBoundaryThatSeesANewStamp(t *testing.T) {
	path, captured, _, reads := recentSealFixture(t, "first")
	var request requestFileDigests
	request.deferRecentReads()
	if _, err := request.current(path, captured); err != nil {
		t.Fatal(err)
	}
	ctimeTick(t, path)
	if err := os.WriteFile(path, []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := request.current(path, captured)
	if err != nil || got != sha256.Sum256([]byte("other")) || *reads != 1 {
		t.Fatalf("boundary after a stamp-moving write = %x, %v, reads %d", got, err, *reads)
	}
}

func TestSealingRequestStillSealsAfterIdenticalRewrite(t *testing.T) {
	path, captured, _, reads := recentSealFixture(t, "first")
	var request requestFileDigests
	request.deferRecentReads()
	if _, err := request.current(path, captured); err != nil {
		t.Fatal(err)
	}
	ctimeTick(t, path)
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := request.current(path, captured); err != nil || got != captured {
		t.Fatalf("boundary after an identical rewrite = %x, %v", got, err)
	}
	stamp, _ := filestamp.Stat(path)
	holdStamp(path, stamp)
	if err := os.WriteFile(path, []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := request.seal(); err == nil {
		t.Fatalf("seal accepted a same-stamp edit after an identical rewrite (%d reads)", *reads)
	}
}

func TestSealedDigestDoesNotDecideALaterPhase(t *testing.T) {
	path, captured, stamp, reads := recentSealFixture(t, "first")
	holdStamp(path, stamp)
	var request requestFileDigests
	request.deferRecentReads()
	if _, err := request.current(path, captured); err != nil {
		t.Fatal(err)
	}
	if err := request.seal(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	request.deferRecentReads()
	if _, err := request.current(path, captured); err != nil {
		t.Fatal(err)
	}
	if err := request.seal(); err == nil || *reads != 2 {
		t.Fatalf("second phase seal = %v after %d reads, want a mismatch on its own read", err, *reads)
	}
}

func TestSealingRequestProvesOldFileAtFirstBoundary(t *testing.T) {
	skipWithoutStamps(t)
	trustedClock(t)
	path := filepath.Join(t.TempDir(), "A.cls")
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	reads := countReads(t, path)
	captured := sha256.Sum256([]byte("first"))
	var request requestFileDigests
	request.deferRecentReads()
	for range 3 {
		if got, err := request.current(path, captured); err != nil || got != captured {
			t.Fatalf("boundary digest = %x, %v", got, err)
		}
	}
	if err := request.seal(); err != nil || *reads != 1 {
		t.Fatalf("seal = %v, reads %d, want the first boundary's single read", err, *reads)
	}
}

// A request that never seals has no later read to rely on.
func TestUnsealedRequestReadsRecentFileAtEveryBoundary(t *testing.T) {
	path, captured, stamp, reads := recentSealFixture(t, "first")
	holdStamp(path, stamp)
	var request requestFileDigests
	if got, err := request.current(path, captured); err != nil || got != captured {
		t.Fatalf("first boundary = %x, %v", got, err)
	}
	if err := os.WriteFile(path, []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := request.current(path, captured); err != nil || got != sha256.Sum256([]byte("other")) || *reads != 2 {
		t.Fatalf("second boundary = %x, %v, reads %d", got, err, *reads)
	}
}
