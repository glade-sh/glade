package testdaemon

import (
	"bufio"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestGracefulStopClientRetainsCompletedRun(t *testing.T) {
	for _, route := range []string{"cancel", "deadline"} {
		t.Run(route, func(t *testing.T) {
			socket, listener := protocolWiringUnixListener(t)
			defer listener.Close()
			ctx, cancel := context.WithCancel(context.Background())
			if route == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			}
			defer cancel()
			served := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					served <- err
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				reader := bufio.NewReader(conn)
				if _, err := DecodeRequestV1(reader); err != nil {
					served <- err
					return
				}
				if route == "cancel" {
					cancel()
				}
				// EOF on the request half cancels execution while leaving the
				// response half available for its final results.
				_, _ = reader.ReadByte()
				run := gracefulStopRun()
				served <- writeServerResponseV1(conn, ResponseV1{Version: ProtocolVersionV1, Op: OpRunResult, OK: true, Run: &run})
			}()
			response, err := RunV1(ctx, socket, RunRequestV1{Parallelism: 1})
			if err != nil || response.Run == nil || response.Run.Summary().Passed != 1 {
				t.Errorf("final response lost completed test: response=%#v err=%v", response, err)
			}
			if response.Op != OpRunResult || !response.OK {
				t.Errorf("final run changed response operation: %#v", response)
			}
			if err := <-served; err != nil {
				t.Errorf("final response write: %v", err)
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
