package testdaemon

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

var (
	serverReadIOTimeoutV1  = 10 * time.Second
	serverWriteIOTimeoutV1 = 10 * time.Second
)

type ServerConfig struct {
	Root   string
	Socket string
	Watch  bool
	Warm   bool
}

type Server struct {
	daemon        *Daemon
	socket        string
	pidPath       string
	pidRoot       *os.Root
	listener      net.Listener
	watchOn       bool
	defaultSocket bool

	warmMu   sync.Mutex
	warmDone chan struct{}
	warmErr  error
	warming  bool
	ready    bool

	runOnce      sync.Once
	runAdmission chan struct{}
	runExecution chan struct{}
	runRequestV1 func(context.Context, RunRequestV1) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error)

	captureScopeFn      func(watch.Scope) (watch.Snapshot, error)
	newBackendWatcherFn func(context.Context, watch.Config, watch.Snapshot) (watch.BackendWatcher, watch.Backend, error)
	afterWatchUpdateFn  func()
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

	privateBase, privateRelative := privateServeDir(root)
	pidRoot, err := openPrivateDaemonDir(privateBase, privateRelative)
	if err != nil {
		return nil, err
	}
	keepPIDRoot := false
	defer func() {
		if !keepPIDRoot {
			_ = pidRoot.Close()
		}
	}()
	defaultSocket := socket == ServeSocketPath(root)
	if !defaultSocket {
		if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
			return nil, err
		}
	}
	if err := claimServeSocket(socket, pidPath, pidRoot, defaultSocket); err != nil {
		return nil, err
	}
	removeServeSocket(socket, pidRoot, defaultSocket)

	daemon, err := New(root)
	if err != nil {
		return nil, err
	}

	s := &Server{
		daemon:        daemon,
		socket:        socket,
		pidPath:       pidPath,
		pidRoot:       pidRoot,
		watchOn:       cfg.Watch,
		defaultSocket: defaultSocket,
		warmDone:      make(chan struct{}),
	}
	keepPIDRoot = true
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
	if err := chmodServeSocket(s.socket, s.pidRoot, s.defaultSocket); err != nil {
		_ = listener.Close()
		removeServeSocket(s.socket, s.pidRoot, s.defaultSocket)
		return fmt.Errorf("restrict test server socket: %w", err)
	}
	s.listener = listener
	if err := writePID(s.pidRoot, filepath.Base(s.pidPath), s.pidPath); err != nil {
		_ = listener.Close()
		removeServeSocket(s.socket, s.pidRoot, s.defaultSocket)
		return fmt.Errorf("publish test server PID: %w", err)
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
			go func() {
				if err := s.serveConn(ctx, conn); err != nil && log != nil {
					fmt.Fprintf(log, "glade test serve: connection error: %v\n", err)
				}
			}()
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
	removeServeSocket(s.socket, s.pidRoot, s.defaultSocket)
	if s.pidRoot != nil {
		_ = s.pidRoot.Remove(filepath.Base(s.pidPath))
		_ = s.pidRoot.Close()
	}
	return nil
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) error {
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(serverReadIOTimeoutV1)); err != nil {
		return fmt.Errorf("set test daemon request read deadline: %w", err)
	}
	reader := bufio.NewReader(conn)
	var request RequestV1
	if err := decodeProtocolV1Frame(reader, &request); err != nil {
		if clearErr := conn.SetReadDeadline(time.Time{}); clearErr != nil {
			return fmt.Errorf("clear test daemon request read deadline: %w", clearErr)
		}
		return writeServerResponseV1(conn, ResponseV1{
			Version: ProtocolVersionV1,
			Op:      OpError,
			ID:      request.ID,
			OK:      false,
			Error:   err.Error(),
		})
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return fmt.Errorf("clear test daemon request read deadline: %w", err)
	}
	if err := validateProtocolVersionV1(request.Version); err != nil {
		return writeServerResponseV1(conn, ResponseV1{
			Version: ProtocolVersionV1,
			Op:      OpError,
			ID:      request.ID,
			OK:      false,
			Error:   err.Error(),
		})
	}
	if request.Op != OpRun {
		return writeServerResponseV1(conn, s.handleV1(ctx, request))
	}
	runCtx, cancelRun := context.WithCancel(ctx)
	stopMonitor := monitorRunConnectionV1(conn, reader, cancelRun)
	response := s.handleV1(runCtx, request)
	if err := stopMonitor(); err != nil {
		cancelRun()
		return err
	}
	cancelRun()
	return writeServerResponseV1(conn, response)
}

func monitorRunConnectionV1(conn net.Conn, reader *bufio.Reader, cancel context.CancelFunc) func() error {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = reader.ReadByte()
		select {
		case <-stop:
			return
		default:
			cancel()
		}
	}()
	return func() error {
		close(stop)
		if err := conn.SetReadDeadline(time.Now()); err != nil {
			return fmt.Errorf("stop test daemon disconnect monitor: %w", err)
		}
		<-done
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return fmt.Errorf("clear test daemon disconnect monitor deadline: %w", err)
		}
		return nil
	}
}

func writeServerResponseV1(conn net.Conn, response ResponseV1) error {
	var frame bytes.Buffer
	if err := EncodeResponseV1(&frame, response); err != nil {
		frame.Reset()
		fallback := ResponseV1{
			Version: ProtocolVersionV1,
			Op:      OpError,
			ID:      response.ID,
			OK:      false,
			Error:   fmt.Sprintf("test daemon response could not be encoded within the maximum protocol frame size: %v", err),
		}
		if fallbackErr := EncodeResponseV1(&frame, fallback); fallbackErr != nil {
			return fmt.Errorf("encode compact test daemon response error: %w", fallbackErr)
		}
	}
	if err := conn.SetWriteDeadline(time.Now().Add(serverWriteIOTimeoutV1)); err != nil {
		return fmt.Errorf("set test daemon response write deadline: %w", err)
	}
	data := frame.Bytes()
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return fmt.Errorf("write test daemon response: %w", err)
		}
		if n <= 0 {
			return fmt.Errorf("write test daemon response: %w", io.ErrShortWrite)
		}
		data = data[n:]
	}
	return nil
}

func (s *Server) handleV1(ctx context.Context, request RequestV1) ResponseV1 {
	response := ResponseV1{
		Version: ProtocolVersionV1,
		ID:      request.ID,
		OK:      false,
	}
	switch request.Op {
	case OpPing:
		if request.Run != nil {
			response.Op = OpError
			response.Error = "ping request cannot include run policy"
			return response
		}
		ready, warming := s.status()
		response.Op = OpPong
		response.OK = true
		response.Ready = ready
		response.Warming = warming
		response.Project = s.daemon.Root()
		return response
	case OpShutdown:
		if request.Run != nil {
			response.Op = OpError
			response.Error = "shutdown request cannot include run policy"
			return response
		}
		go func() { _ = s.Close() }()
		response.Op = OpShutdownAck
		response.OK = true
		return response
	case OpRun:
		response.Op = OpError
		if request.Run == nil {
			response.Error = "run request requires run policy"
			return response
		}
		if err := validateRunRequestV1(*request.Run); err != nil {
			response.Error = err.Error()
			return response
		}
		release, err := s.acquireRun(ctx)
		if err != nil {
			response.Error = err.Error()
			return response
		}
		defer release()
		if err := s.waitReady(ctx); err != nil {
			response.Error = err.Error()
			return response
		}
		runRequest := s.runRequestV1
		if runRequest == nil {
			runRequest = s.daemon.runRequestV1
		}
		if err := ctx.Err(); err != nil {
			response.Error = err.Error()
			return response
		}
		run, selection, shardPlan, err := runRequest(ctx, *request.Run)
		if err != nil {
			response.Error = err.Error()
			return response
		}
		response.Op = OpRunResult
		response.OK = true
		response.Run = &run
		response.Selection = &selection
		response.ShardPlan = shardPlan
		return response
	default:
		response.Op = OpError
		response.Error = fmt.Sprintf("unknown op %q", request.Op)
		return response
	}
}

func (s *Server) acquireRun(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.runOnce.Do(func() {
		s.runAdmission = make(chan struct{}, 2)
		s.runExecution = make(chan struct{}, 1)
	})
	select {
	case s.runAdmission <- struct{}{}:
	default:
		return nil, errors.New("test server is busy; retry the request")
	}
	if err := ctx.Err(); err != nil {
		<-s.runAdmission
		return nil, err
	}
	select {
	case s.runExecution <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-s.runExecution
			<-s.runAdmission
			return nil, err
		}
		return func() {
			<-s.runExecution
			<-s.runAdmission
		}, nil
	case <-ctx.Done():
		<-s.runAdmission
		return nil, ctx.Err()
	}
}

func (s *Server) watchLoop(ctx context.Context, root string) {
	captureScope := s.captureScopeFn
	if captureScope == nil {
		captureScope = watch.CaptureScope
	}
	newBackendWatcher := s.newBackendWatcherFn
	if newBackendWatcher == nil {
		newBackendWatcher = watch.NewBackendWatcher
	}
	afterWatchUpdate := s.afterWatchUpdateFn
	if afterWatchUpdate == nil {
		afterWatchUpdate = func() {
			apextest.InvalidateRuntimeCaches()
			s.daemon.Warm()
		}
	}
	cfg := watch.Config{Root: root}.Normalized()
	var watcher watch.BackendWatcher
	var watchCancel context.CancelFunc
	for {
		if ctx.Err() != nil {
			return
		}
		var candidate watch.BackendWatcher
		var candidateCancel context.CancelFunc
		var candidateScope watch.Scope
		err := s.daemon.ReloadPreparedStable(cfg.Scope, captureScope, func(_ project.Project, scope watch.Scope, baseline watch.Snapshot) error {
			watchCtx, cancel := context.WithCancel(ctx)
			started, _, startErr := newBackendWatcher(watchCtx, watch.Config{Root: root, Scope: scope}.Normalized(), baseline)
			if startErr != nil {
				cancel()
				return startErr
			}
			candidate = started
			candidateCancel = cancel
			candidateScope = scope
			return nil
		})
		if err == nil {
			if ctx.Err() != nil {
				if candidateCancel != nil {
					candidateCancel()
				}
				if candidate != nil {
					_ = candidate.Close()
				}
				return
			}
			watcher = candidate
			watchCancel = candidateCancel
			cfg.Scope = candidateScope
			break
		}
		if candidateCancel != nil {
			candidateCancel()
		}
		if candidate != nil {
			_ = candidate.Close()
		}
		var drift *WatchStateDriftError
		if !errors.As(err, &drift) {
			return
		}
	}
	defer func() {
		watchCancel()
		_ = watcher.Close()
	}()
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
			exact, err := s.daemon.TryUpdateChanges(ctx, changes, cfg.Scope)
			if err != nil {
				continue
			}
			if !exact {
				var candidate watch.BackendWatcher
				var candidateCancel context.CancelFunc
				var candidateScope watch.Scope
				var err error
				for {
					candidate = nil
					candidateCancel = nil
					err = s.daemon.ReloadPreparedStable(cfg.Scope, captureScope, func(_ project.Project, stableScope watch.Scope, candidateSnapshot watch.Snapshot) error {
						candidateScope = stableScope
						candidateCtx, cancel := context.WithCancel(ctx)
						candidateWatcher, _, prepareErr := newBackendWatcher(candidateCtx, watch.Config{Root: root, Scope: candidateScope}.Normalized(), candidateSnapshot)
						if prepareErr != nil {
							cancel()
							return prepareErr
						}
						candidate = candidateWatcher
						candidateCancel = cancel
						return nil
					})
					var drift *WatchStateDriftError
					if !errors.As(err, &drift) || ctx.Err() != nil {
						break
					}
					if candidateCancel != nil {
						candidateCancel()
					}
					if candidate != nil {
						_ = candidate.Close()
					}
				}
				if err != nil {
					if candidateCancel != nil {
						candidateCancel()
					}
					if candidate != nil {
						_ = candidate.Close()
					}
					continue
				}
				oldWatcher := watcher
				oldCancel := watchCancel
				watcher = candidate
				watchCancel = candidateCancel
				cfg.Scope = candidateScope
				oldCancel()
				_ = oldWatcher.Close()
			}
			afterWatchUpdate()
		}
	}
}

func writePID(root *os.Root, name, path string) error {
	if root == nil {
		return errors.New("PID directory is not open")
	}
	if name == "." || name == string(os.PathSeparator) {
		return fmt.Errorf("invalid PID path %q", path)
	}
	if info, err := root.Lstat(name); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("PID path %q must not be a symlink", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp, tmpName, err := createDaemonTemp(root)
	if err != nil {
		return err
	}
	defer root.Remove(tmpName)
	if _, err := fmt.Fprintf(tmp, "%d\n", os.Getpid()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return root.Rename(tmpName, name)
}

func openPrivateDaemonDir(base, relative string) (*os.Root, error) {
	cleanRelative := filepath.Clean(relative)
	if filepath.IsAbs(cleanRelative) || cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("daemon directory %q must stay within %q", relative, base)
	}
	current, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(cleanRelative, string(os.PathSeparator)) {
		if err := current.Mkdir(component, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			_ = current.Close()
			return nil, err
		}
		beforeOpen, err := current.Lstat(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		if beforeOpen.Mode()&os.ModeSymlink != 0 {
			_ = current.Close()
			return nil, fmt.Errorf("daemon directory component %q must not be a symlink", component)
		}
		if !beforeOpen.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("daemon path component %q is not a directory", component)
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
			return nil, fmt.Errorf("daemon directory component %q changed while opening", component)
		}
		_ = current.Close()
		current = next
	}
	if runtime.GOOS != "windows" {
		if err := current.Chmod(".", 0o700); err != nil {
			_ = current.Close()
			return nil, fmt.Errorf("restrict daemon directory %q: %w", relative, err)
		}
	}
	return current, nil
}

func createDaemonTemp(root *os.Root) (*os.File, string, error) {
	var random [16]byte
	for range 100 {
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := ".serve-pid-" + hex.EncodeToString(random[:])
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return file, name, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("could not allocate unique daemon PID temporary file")
}

func chmodServeSocket(socket string, root *os.Root, useRoot bool) error {
	if useRoot {
		info, err := root.Lstat(filepath.Base(socket))
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("test server socket %q is not a socket", socket)
		}
		return root.Chmod(filepath.Base(socket), 0o600)
	}
	return os.Chmod(socket, 0o600)
}

func removeServeSocket(socket string, root *os.Root, useRoot bool) {
	if useRoot && root != nil {
		_ = root.Remove(filepath.Base(socket))
		return
	}
	_ = os.Remove(socket)
}

func claimServeSocket(socket, pidPath string, pidRoot *os.Root, defaultSocket bool) error {
	pidFile, err := pidRoot.Open(filepath.Base(pidPath))
	if err != nil {
		return nil
	}
	data, readErr := io.ReadAll(pidFile)
	closeErr := pidFile.Close()
	if readErr != nil || closeErr != nil {
		return nil
	}
	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil || pid <= 0 {
		return nil
	}
	if processAlive(pid) {
		return fmt.Errorf("test server already running (pid %d)", pid)
	}
	removeServeSocket(socket, pidRoot, defaultSocket)
	_ = pidRoot.Remove(filepath.Base(pidPath))
	return nil
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
