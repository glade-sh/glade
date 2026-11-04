package testdaemon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestGracefulStopClientRetainsCompletedRun(t *testing.T) {
	t.Run("RunV1", func(t *testing.T) {
		for _, mode := range []string{"cancel", "deadline", "deadline-controlled"} {
			t.Run(mode, func(t *testing.T) { gracefulStopRunV1RetainsCompletedRun(t, mode) })
		}
	})
	for _, route := range []string{"cancel", "deadline", "deadline-controlled"} {
		t.Run(route, func(t *testing.T) {
			socket, listener := protocolWiringUnixListener(t)
			defer listener.Close()
			client, err := net.DialTimeout("unix", socket, 10*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if unix, ok := listener.(*net.UnixListener); ok {
				if err := unix.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			peer, err := listener.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			ctx, cancel := context.WithCancel(context.Background())
			var clock *manualDeadlineContext
			if route == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			} else if route == "deadline-controlled" {
				cancel()
				clock = newManualDeadlineContext(100 * time.Millisecond)
				ctx, cancel = clock, clock.Close
			}
			defer cancel()
			ready := make(chan error, 1)
			served := make(chan error, 1)
			serverExited := make(chan struct{})
			clientExited := make(chan struct{})
			go func() {
				defer close(serverExited)
				_ = peer.SetDeadline(time.Now().Add(30 * time.Second))
				reader := bufio.NewReader(peer)
				if _, err := DecodeRequestV1(reader); err != nil {
					ready <- err
					served <- err
					return
				}
				ready <- nil
				if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
					served <- fmt.Errorf("request half-close: %w", err)
					return
				}
				run := gracefulStopRun()
				served <- writeServerResponseV1(peer, ResponseV1{Version: ProtocolVersionV1, Op: OpRunResult, OK: true, Run: &run})
			}()
			t.Cleanup(func() {
				_ = client.Close()
				_ = peer.Close()
				for _, done := range []<-chan struct{}{serverExited, clientExited} {
					select {
					case <-done:
					case <-time.After(20 * time.Second):
						t.Error("protocol worker did not exit")
					}
				}
			})
			type outcome struct {
				response ResponseV1
				err      error
				cause    error
			}
			finished := make(chan outcome, 1)
			go func() {
				defer close(clientExited)
				response, err := callV1OnConn(ctx, client, RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &RunRequestV1{Parallelism: 1}})
				finished <- outcome{response, err, ctx.Err()}
			}()
			select {
			case err := <-ready:
				if err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if err := ctx.Err(); err != nil {
					t.Fatalf("context expired before request decoded: %v", err)
				}
			case result := <-finished:
				t.Fatalf("client finished before request decoded: %#v", result)
			case <-time.After(20 * time.Second):
				t.Fatal("request was not decoded")
			}
			if route == "cancel" {
				cancel()
			} else if route == "deadline-controlled" {
				clock.Advance(100 * time.Millisecond)
			}
			select {
			case result := <-finished:
				response, err := result.response, result.err
				if err != nil || response.Run == nil || response.Run.Summary().Passed != 1 {
					t.Errorf("final response lost completed test: response=%#v err=%v", response, err)
				}
				if response.Op != OpRunResult || !response.OK {
					t.Errorf("final run changed response operation: %#v", response)
				}
				if route != "cancel" && !errors.Is(result.cause, context.DeadlineExceeded) {
					t.Errorf("deadline cause at return = %v", result.cause)
				}
			case <-time.After(20 * time.Second):
				t.Fatal("client did not return")
			}
			select {
			case err := <-served:
				if err != nil {
					t.Errorf("final response write: %v", err)
				}
			case <-time.After(20 * time.Second):
				t.Fatal("server did not return")
			}
		})
	}
}

func TestGracefulStopServerRetainsPartialRunOnCancellation(t *testing.T) {
	for _, stopErr := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(stopErr.Error(), func(t *testing.T) {
			server := protocolWiringReadyServer(t)
			server.runRequestV1 = func(context.Context, RunRequestV1) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error) {
				return gracefulStopRun(), watch.TestSelection{}, nil, stopErr
			}
			response := server.handleV1(context.Background(), RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &RunRequestV1{Parallelism: 1}})
			if response.Run == nil || response.Run.Summary().Passed != 1 || response.OK || response.Error != stopErr.Error() {
				t.Fatalf("partial run discarded: %#v", response)
			}
		})
	}
}

func TestGracefulStopClientBoundsUnresponsiveRun(t *testing.T) {
	socket, listener := protocolWiringUnixListener(t)
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	done := make(chan error, 1)
	go func() { _, err := RunV1(ctx, socket, RunRequestV1{Parallelism: 1}); done <- err }()
	conn := <-accepted
	defer conn.Close()
	if _, err := DecodeRequestV1(bufio.NewReader(conn)); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unresponsive cancellation: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("cancelled run did not bound final-response wait")
	}
}

func gracefulStopRun() testreport.Run {
	return testreport.Run{Suites: []testreport.Suite{{Name: "StoppableTest", Cases: []testreport.Case{
		{ClassName: "StoppableTest", MethodName: "aPass", Status: testreport.StatusPass},
		{ClassName: "StoppableTest", MethodName: "zWait", Status: testreport.StatusUnsupported, Reason: testreport.ReasonCancelled, Problem: &testreport.Problem{Type: "Canceled", Message: "context canceled"}},
	}}}}
}

// Exercise the public dialing route while retaining a completed response after cancellation.
func gracefulStopRunV1RetainsCompletedRun(t *testing.T, mode string) {
	socket, listener := protocolWiringUnixListener(t)
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	var clock *manualDeadlineContext
	if mode == "deadline" {
		cancel()
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	} else if mode == "deadline-controlled" {
		cancel()
		clock = newManualDeadlineContext(100 * time.Millisecond)
		ctx, cancel = dialContextWithoutDeadline{clock}, clock.Close
	}
	t.Cleanup(cancel)
	ready := make(chan error, 1)
	served := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			ready <- err
			served <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
		reader := bufio.NewReader(conn)
		_, err = DecodeRequestV1(reader)
		ready <- err
		if err != nil {
			served <- err
			return
		}
		if _, err = reader.ReadByte(); !errors.Is(err, io.EOF) {
			served <- fmt.Errorf("request half-close: %v", err)
			return
		}
		run := gracefulStopRun()
		served <- writeServerResponseV1(conn, ResponseV1{Version: ProtocolVersionV1, Op: OpRunResult, OK: true, Run: &run})
	}()
	type outcome struct {
		response ResponseV1
		err      error
		cause    error
	}
	finished := make(chan outcome, 1)
	go func() {
		response, err := RunV1(ctx, socket, RunRequestV1{Parallelism: 1})
		finished <- outcome{response, err, ctx.Err()}
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("decode request: %v", err)
		}
	case result := <-finished:
		t.Fatalf("client returned before request decoded: %#v", result)
	case <-time.After(20 * time.Second):
		t.Fatal("request not decoded")
	}
	if mode == "deadline-controlled" {
		clock.Advance(100 * time.Millisecond)
	} else if mode == "cancel" {
		cancel()
	}
	select {
	case result := <-finished:
		if mode != "cancel" && !errors.Is(result.cause, context.DeadlineExceeded) {
			t.Errorf("deadline cause at return = %v", result.cause)
		}
		if result.err != nil || !result.response.OK || result.response.Op != OpRunResult || result.response.Run == nil || result.response.Run.Summary().Passed != 1 {
			t.Fatalf("completed response lost: %#v", result)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("client did not finish")
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("server: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("server did not finish")
	}
}
