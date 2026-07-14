package testdaemon

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
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

	runOnce      sync.Once
	runAdmission chan struct{}
	runExecution chan struct{}
	runRequestV1 func(context.Context, RunRequestV1) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error)
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
	_ = os.Remove(s.socket)
	_ = os.Remove(s.pidPath)
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
