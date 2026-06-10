package testdaemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

type ServerConfig struct {
	Root   string
	Socket string
	Watch  bool
	Warm   bool
}

type Server struct {
	daemon   *Daemon
	socket   string
	pidPath  string
	listener net.Listener
	watchOn  bool

	warmMu   sync.Mutex
	warmDone chan struct{}
	warmErr  error
	warming  bool
	ready    bool

	runMu sync.Mutex
}

func NewServer(cfg ServerConfig) (*Server, error) {
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	socket := cfg.Socket
	if socket == "" {
		socket = ServeSocketPath(root)
	}
	socket, err = filepath.Abs(socket)
	if err != nil {
		return nil, err
	}
	pidPath := ServePIDPath(root)

	if err := claimServeSocket(socket, pidPath); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		return nil, err
	}
	_ = os.Remove(socket)

	daemon, err := New(root)
	if err != nil {
		return nil, err
	}

	s := &Server{
		daemon:   daemon,
		socket:   socket,
		pidPath:  pidPath,
		watchOn:  cfg.Watch,
		warmDone: make(chan struct{}),
	}
	if cfg.Warm {
		// Warm begins in ListenAndServe so status logs reach the user.
	} else {
		s.ready = true
		close(s.warmDone)
	}
	return s, nil
}

func (s *Server) startWarm(log io.Writer) {
	s.warmMu.Lock()
	s.warming = true
	s.warmMu.Unlock()
	if log != nil {
		fmt.Fprintln(log, "glade test serve: warming test runtime...")
	}
	go func() {
		s.daemon.Warm()
		s.warmMu.Lock()
		s.warming = false
		s.ready = true
		s.warmMu.Unlock()
		if log != nil {
			fmt.Fprintln(log, "glade test serve: runtime ready")
		}
		close(s.warmDone)
	}()
}

func (s *Server) waitReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.warmDone:
		s.warmMu.Lock()
		err := s.warmErr
		s.warmMu.Unlock()
		return err
	}
}

func (s *Server) status() (ready, warming bool) {
	s.warmMu.Lock()
	defer s.warmMu.Unlock()
	return s.ready, s.warming
}

func (s *Server) ListenAndServe(ctx context.Context, log io.Writer) error {
	defer s.Close()
	if s.watchOn {
		go s.watchLoop(ctx, s.daemon.Root())
	}
	if !s.ready && !s.warming {
		s.startWarm(log)
	}
	listener, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}
	s.listener = listener
	if err := writePID(s.pidPath); err != nil && log != nil {
		fmt.Fprintf(log, "glade test serve: warning: could not write pid file: %v\n", err)
	}
	if log != nil {
		fmt.Fprintf(log, "glade test serve: listening on %s\n", s.socket)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	errCh := make(chan error, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
					errCh <- nil
				} else {
					errCh <- err
				}
				return
			}
			go s.serveConn(ctx, conn)
		}
	}()

	select {
	case <-ctx.Done():
		_ = listener.Close()
		return nil
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	case <-sigCh:
		return nil
	}
}

func (s *Server) Close() error {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	_ = os.Remove(s.socket)
	_ = os.Remove(s.pidPath)
	return nil
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return
	}
	var req Request
	if err := json.Unmarshal(trimLine(line), &req); err != nil {
		_ = writeResponse(conn, Response{Op: OpError, OK: false, Error: err.Error()})
		return
	}
	resp := s.handle(ctx, req)
	_ = writeResponse(conn, resp)
}

func (s *Server) handle(ctx context.Context, req Request) Response {
	switch req.Op {
	case OpPing:
		ready, warming := s.status()
		return Response{
			Op:      OpPong,
			OK:      true,
			Ready:   ready,
			Warming: warming,
			Project: s.daemon.Root(),
		}
	case OpShutdown:
		go func() { _ = s.Close() }()
		return Response{Op: OpShutdownAck, OK: true}
	case OpRun:
		if err := s.waitReady(ctx); err != nil {
			return Response{Op: OpError, ID: req.ID, OK: false, Error: err.Error()}
		}
		s.runMu.Lock()
		defer s.runMu.Unlock()
		opts := apextest.Options{Filter: req.Filter}
		var (
			run       testreport.Run
			selection watch.TestSelection
			err       error
		)
		if trimmed := strings.TrimSpace(req.ChangedSince); trimmed != "" {
			run, selection, err = s.daemon.RunChangedSinceOptions(trimmed, opts)
		} else {
			run = s.daemon.RunOptions(opts)
		}
		if err != nil {
			return Response{Op: OpError, ID: req.ID, OK: false, Error: err.Error()}
		}
		return Response{
			Op:        OpRunResult,
			ID:        req.ID,
			OK:        true,
			Run:       &run,
			Selection: &selection,
		}
	default:
		return Response{Op: OpError, ID: req.ID, OK: false, Error: fmt.Sprintf("unknown op %q", req.Op)}
	}
}

func (s *Server) watchLoop(ctx context.Context, root string) {
	cfg := watch.Config{Root: root}.Normalized()
	previous, err := watch.CaptureSnapshot(root)
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	watcher, _, err := watch.NewBackendWatcher(ctx, cfg, previous)
	if err != nil {
		return
	}
	defer watcher.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-watcher.Errors():
			if !ok {
				return
			}
		case changes, ok := <-watcher.Changes():
			if !ok {
				return
			}
			if len(changes) == 0 {
				continue
			}
			if err := s.daemon.UpdateChanges(changes); err != nil {
				continue
			}
			apextest.InvalidateRuntimeCaches()
			s.daemon.Warm()
		}
	}
}

func writeResponse(w io.Writer, resp Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func writePID(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
}

func claimServeSocket(socket, pidPath string) error {
	pid, err := readPID(pidPath)
	if err != nil {
		return nil
	}
	if processAlive(pid) {
		return fmt.Errorf("test server already running (pid %d)", pid)
	}
	_ = os.Remove(socket)
	_ = os.Remove(pidPath)
	return nil
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func trimLine(line []byte) []byte {
	return bytesTrimNewline(line)
}

func bytesTrimNewline(line []byte) []byte {
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	return line
}
