package dap

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func startCancelServe(t *testing.T, handler *Handler) (*io.PipeWriter, <-chan map[string]any, <-chan error) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	t.Cleanup(func() {
		_ = inW.Close()
		_ = outR.Close()
	})
	done := make(chan error, 1)
	go func() {
		defer outW.Close()
		done <- Serve(inR, outW, handler)
	}()
	messages := make(chan map[string]any, 16)
	go func() {
		reader := NewReader(outR)
		for {
			raw, err := reader.Read()
			if err != nil {
				close(messages)
				return
			}
			var message map[string]any
			if err := json.Unmarshal(raw, &message); err != nil {
				close(messages)
				return
			}
			messages <- message
		}
	}()
	return inW, messages, done
}

func waitForResponse(t *testing.T, messages <-chan map[string]any, command string) map[string]any {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				t.Fatalf("stream closed before %s response", command)
			}
			if message["type"] == MessageTypeResponse && message["command"] == command {
				return message
			}
		case <-timeout:
			t.Fatalf("no %s response", command)
		}
	}
}

func TestServeCancelRequestEndsSlowLaunch(t *testing.T) {
	handler := NewHandler(Snapshot{})
	started := make(chan struct{})
	handler.SetLaunchHandlerContext(func(ctx context.Context, request LaunchRequest) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	inW, messages, _ := startCancelServe(t, handler)

	writeRequest(t, inW, Request{Seq: 1, Type: MessageTypeRequest, Command: CommandInitialize})
	waitForResponse(t, messages, CommandInitialize)
	writeRequest(t, inW, request(2, CommandLaunch, map[string]any{"program": "x"}))
	<-started
	writeRequest(t, inW, request(3, CommandCancel, map[string]any{"requestId": 2}))

	launch := waitForResponse(t, messages, CommandLaunch)
	if launch["success"] != false || launch["message"] != "cancelled" {
		t.Fatalf("launch response = %v", launch)
	}
	cancel := waitForResponse(t, messages, CommandCancel)
	if cancel["success"] != true {
		t.Fatalf("cancel response = %v", cancel)
	}
}

func TestServeDisconnectEndsSlowLaunch(t *testing.T) {
	handler := NewHandler(Snapshot{})
	started := make(chan struct{})
	handler.SetLaunchHandlerContext(func(ctx context.Context, request LaunchRequest) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	inW, messages, done := startCancelServe(t, handler)

	writeRequest(t, inW, request(1, CommandLaunch, map[string]any{"program": "x"}))
	<-started
	writeRequest(t, inW, Request{Seq: 2, Type: MessageTypeRequest, Command: CommandDisconnect})

	launch := waitForResponse(t, messages, CommandLaunch)
	if launch["success"] != false {
		t.Fatalf("launch response = %v", launch)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DAP server did not stop after disconnect")
	}
}

func TestCancelBeforeLaunchStartsFailsQueuedLaunch(t *testing.T) {
	handler := NewHandler(Snapshot{})
	releaseFirst := make(chan struct{})
	started := make(chan struct{})
	handler.SetLaunchHandlerContext(func(ctx context.Context, request LaunchRequest) error {
		if request.Program == "first" {
			close(started)
			<-releaseFirst
			return nil
		}
		return ctx.Err()
	})
	inW, messages, _ := startCancelServe(t, handler)

	writeRequest(t, inW, request(1, CommandLaunch, map[string]any{"program": "first"}))
	<-started
	// Seq 2 waits behind seq 1 in the queue; the cancel is read before it starts.
	writeRequest(t, inW, request(2, CommandLaunch, map[string]any{"program": "second"}))
	writeRequest(t, inW, request(3, CommandCancel, map[string]any{"requestId": 2}))
	time.Sleep(100 * time.Millisecond)
	close(releaseFirst)

	first := waitForResponse(t, messages, CommandLaunch)
	if first["success"] != true {
		t.Fatalf("first launch = %v", first)
	}
	second := waitForResponse(t, messages, CommandLaunch)
	if second["success"] != false || second["message"] != "cancelled" {
		t.Fatalf("queued launch should be cancelled, got %v", second)
	}
}

func TestCancelRequestForOtherRequestLeavesLaunchRunning(t *testing.T) {
	handler := NewHandler(Snapshot{})
	release := make(chan struct{})
	started := make(chan struct{})
	handler.SetLaunchHandlerContext(func(ctx context.Context, request LaunchRequest) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	inW, messages, _ := startCancelServe(t, handler)

	writeRequest(t, inW, request(1, CommandLaunch, map[string]any{"program": "x"}))
	<-started
	writeRequest(t, inW, request(2, CommandCancel, map[string]any{"requestId": 99}))
	time.Sleep(50 * time.Millisecond)
	close(release)
	launch := waitForResponse(t, messages, CommandLaunch)
	if launch["success"] != true {
		t.Fatalf("launch response = %v", launch)
	}
}
