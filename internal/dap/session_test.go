package dap

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

type signalingRequestReader struct {
	read chan struct{}
}

func (r *signalingRequestReader) ReadRequest() (Request, error) {
	close(r.read)
	return Request{Seq: 1, Type: MessageTypeRequest, Command: CommandInitialize}, nil
}

func TestReadRequestPumpStopsWhenSessionIsDone(t *testing.T) {
	reader := &signalingRequestReader{read: make(chan struct{})}
	requests := make(chan requestReadResult)
	sessionDone := make(chan struct{})
	pumpDone := make(chan struct{})
	go func() {
		readRequests(reader, requests, sessionDone)
		close(pumpDone)
	}()

	<-reader.read
	close(sessionDone)

	select {
	case <-pumpDone:
	case <-time.After(time.Second):
		t.Fatal("request reader pump remained blocked after session ended")
	}
}

func TestServePublishesLiveStoppedEvent(t *testing.T) {
	handler := NewHandler(Snapshot{})
	handler.SetLaunchHandler(func(request LaunchRequest) error {
		program, err := vm.CompileAnonymous("Integer x = 1;\nx = x + 1;")
		if err != nil {
			return err
		}
		handler.PrepareLiveSession(vm.New(nil), program)
		return nil
	})

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

	writeRequest(t, inW, Request{Seq: 1, Type: MessageTypeRequest, Command: CommandInitialize})
	writeRequest(t, inW, request(2, CommandSetBreakpoints, map[string]any{
		"source":      map[string]any{"name": "anonymous.apex"},
		"breakpoints": []map[string]any{{"line": 2}},
	}))
	writeRequest(t, inW, request(3, CommandLaunch, map[string]any{"program": "ignored"}))

	waitForEvent(t, messages, "stopped")
	writeRequest(t, inW, Request{Seq: 4, Type: MessageTypeRequest, Command: CommandDisconnect})

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("DAP server did not stop")
	}
}

func TestServeWaitsForConfigurationDoneBeforeLiveLaunch(t *testing.T) {
	handler := NewHandler(Snapshot{})
	handler.SetLaunchHandler(func(request LaunchRequest) error {
		program, err := vm.CompileAnonymous("Integer x = 1;\nx = x + 1;")
		if err != nil {
			return err
		}
		handler.PrepareLiveSession(vm.New(nil), program)
		return nil
	})

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

	writeRequest(t, inW, Request{Seq: 1, Type: MessageTypeRequest, Command: CommandInitialize})
	writeRequest(t, inW, request(2, CommandLaunch, map[string]any{"program": "ignored"}))
	writeRequest(t, inW, request(3, CommandSetBreakpoints, map[string]any{
		"source":      map[string]any{"name": "anonymous.apex"},
		"breakpoints": []map[string]any{{"line": 2}},
	}))
	writeRequest(t, inW, Request{Seq: 4, Type: MessageTypeRequest, Command: CommandConfigurationDone})

	waitForEvent(t, messages, "stopped")
	writeRequest(t, inW, Request{Seq: 5, Type: MessageTypeRequest, Command: CommandDisconnect})

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("DAP server did not stop")
	}
}

func TestServePublishesSystemDebugOutputEvent(t *testing.T) {
	handler := NewHandler(Snapshot{})
	handler.SetLaunchHandler(func(request LaunchRequest) error {
		program, err := vm.CompileAnonymous("System.debug('dap console');")
		if err != nil {
			return err
		}
		machine := vm.New(nil)
		machine.SetTraceEnabled(true)
		handler.PrepareLiveSession(machine, program)
		return nil
	})

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

	writeRequest(t, inW, Request{Seq: 1, Type: MessageTypeRequest, Command: CommandInitialize})
	writeRequest(t, inW, request(2, CommandLaunch, map[string]any{"program": "ignored"}))
	writeRequest(t, inW, Request{Seq: 3, Type: MessageTypeRequest, Command: CommandConfigurationDone})

	waitForOutput(t, messages, "console", "USER_DEBUG|dap console")
	writeRequest(t, inW, Request{Seq: 4, Type: MessageTypeRequest, Command: CommandDisconnect})

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("DAP server did not stop")
	}
}

func writeRequest(t *testing.T, w io.Writer, request Request) {
	t.Helper()
	if err := Write(w, request); err != nil {
		t.Fatal(err)
	}
}

func waitForEvent(t *testing.T, messages <-chan map[string]any, event string) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				t.Fatalf("message stream closed before %q event", event)
			}
			if message["type"] == MessageTypeEvent && message["event"] == event {
				return
			}
		case <-deadline:
			t.Fatalf("timeout waiting for %q event", event)
		}
	}
}

func waitForOutput(t *testing.T, messages <-chan map[string]any, category string, contains string) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				t.Fatalf("message stream closed before output containing %q", contains)
			}
			if message["type"] != MessageTypeEvent || message["event"] != "output" {
				continue
			}
			body, ok := message["body"].(map[string]any)
			if !ok {
				continue
			}
			if body["category"] == category && strings.Contains(fmt.Sprint(body["output"]), contains) {
				return
			}
		case <-deadline:
			t.Fatalf("timeout waiting for output containing %q", contains)
		}
	}
}
