package testdaemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

const defaultDialTimeout = 3 * time.Second

// Ping checks whether a test server is accepting requests on socket.
func Ping(ctx context.Context, socket string) (Response, error) {
	return call(ctx, socket, Request{Op: OpPing})
}

// Run sends a test run request to the server.
func Run(ctx context.Context, socket string, req Request) (Response, error) {
	req.Op = OpRun
	return call(ctx, socket, req)
}

// Shutdown asks the server to exit.
func Shutdown(ctx context.Context, socket string) error {
	resp, err := call(ctx, socket, Request{Op: OpShutdown})
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	return nil
}

func call(ctx context.Context, socket string, req Request) (Response, error) {
	if socket == "" {
		return Response{}, errors.New("test server socket path is required")
	}
	dialer := net.Dialer{Timeout: defaultDialTimeout}
	conn, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return Response{}, err
	}
	var resp Response
	dec := json.NewDecoder(bufio.NewReader(conn))
	if err := dec.Decode(&resp); err != nil {
		return Response{}, err
	}
	if resp.Op == OpError {
		if resp.Error == "" {
			resp.Error = "test server request failed"
		}
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

// ServerReachable reports whether socket exists and responds to ping.
func ServerReachable(ctx context.Context, socket string) bool {
	ctx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()
	resp, err := Ping(ctx, socket)
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
