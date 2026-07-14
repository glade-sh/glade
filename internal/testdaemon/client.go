package testdaemon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

const defaultDialTimeout = 3 * time.Second

var (
	clientControlIOTimeoutV1 = 3 * time.Second
	clientRunIOTimeoutV1     = time.Hour
)

type ServerUnavailableError struct {
	Socket string
	Err    error
}

func (e *ServerUnavailableError) Error() string {
	return fmt.Sprintf("connect to test server %s: %v", e.Socket, e.Err)
}

func (e *ServerUnavailableError) Unwrap() error { return e.Err }

func IsServerUnavailable(err error) bool {
	var unavailable *ServerUnavailableError
	return errors.As(err, &unavailable)
}

// Ping checks whether a test server is accepting requests on socket.
func Ping(ctx context.Context, socket string) (Response, error) {
	resp, err := PingV1(ctx, socket)
	return responseFromV1(resp), err
}

// Run sends a test run request to the server.
func Run(ctx context.Context, socket string, req Request) (Response, error) {
	resp, err := RunV1(ctx, socket, RunRequestV1{
		Filter:       req.Filter,
		ChangedSince: req.ChangedSince,
		Parallelism:  1,
	})
	return responseFromV1(resp), err
}

// Shutdown asks the server to exit.
func Shutdown(ctx context.Context, socket string) error {
	resp, err := ShutdownV1(ctx, socket)
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	return nil
}

func PingV1(ctx context.Context, socket string) (ResponseV1, error) {
	return callV1(ctx, socket, RequestV1{Version: ProtocolVersionV1, Op: OpPing})
}

func RunV1(ctx context.Context, socket string, run RunRequestV1) (ResponseV1, error) {
	return callV1(ctx, socket, RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &run})
}

func ShutdownV1(ctx context.Context, socket string) (ResponseV1, error) {
	return callV1(ctx, socket, RequestV1{Version: ProtocolVersionV1, Op: OpShutdown})
}

func callV1(ctx context.Context, socket string, req RequestV1) (ResponseV1, error) {
	if socket == "" {
		return ResponseV1{}, errors.New("test server socket path is required")
	}
	dialer := net.Dialer{Timeout: defaultDialTimeout}
	conn, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ResponseV1{}, ctxErr
		}
		return ResponseV1{}, &ServerUnavailableError{Socket: socket, Err: err}
	}
	defer conn.Close()

	timeout := clientControlIOTimeoutV1
	if req.Op == OpRun {
		timeout = clientRunIOTimeoutV1
	}
	deadline := time.Now().Add(timeout)
	callerDeadlineApplied := false
	if callerDeadline, ok := ctx.Deadline(); ok && callerDeadline.Before(deadline) {
		deadline = callerDeadline
		callerDeadlineApplied = true
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return ResponseV1{}, err
	}
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancel()

	if err := EncodeRequestV1(conn, req); err != nil {
		return ResponseV1{}, normalizeClientProtocolErrorV1(ctx, err, deadline, callerDeadlineApplied)
	}
	resp, err := DecodeResponseV1(bufio.NewReader(conn))
	if err != nil {
		return ResponseV1{}, normalizeClientProtocolErrorV1(ctx, err, deadline, callerDeadlineApplied)
	}
	if resp.Op == OpError {
		if resp.Error == "" {
			resp.Error = "test server request failed"
		}
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

func normalizeClientProtocolErrorV1(ctx context.Context, err error, deadline time.Time, callerDeadlineApplied bool) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var netErr net.Error
	if callerDeadlineApplied && errors.As(err, &netErr) && netErr.Timeout() && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return err
}

func responseFromV1(resp ResponseV1) Response {
	return Response{
		Op:        resp.Op,
		ID:        resp.ID,
		OK:        resp.OK,
		Error:     resp.Error,
		Ready:     resp.Ready,
		Warming:   resp.Warming,
		Project:   resp.Project,
		Run:       resp.Run,
		Selection: resp.Selection,
	}
}

// ServerReachable reports whether socket exists and responds to ping.
func ServerReachable(ctx context.Context, socket string) bool {
	ctx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()
	resp, err := PingV1(ctx, socket)
	return err == nil && resp.OK
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid file %s", path)
	}
	return pid, nil
}
