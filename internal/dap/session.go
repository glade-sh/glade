package dap

import (
	"io"
)

type requestReadResult struct {
	request Request
	err     error
}

type requestReader interface {
	ReadRequest() (Request, error)
}

func readRequests(reader requestReader, requests chan<- requestReadResult, done <-chan struct{}) {
	defer close(requests)
	for {
		request, err := reader.ReadRequest()
		select {
		case requests <- requestReadResult{request: request, err: err}:
		case <-done:
			return
		}
		if err != nil {
			return
		}
	}
}

func Serve(r io.Reader, w io.Writer, handler *Handler) error {
	reader := NewReader(r)
	requests := make(chan requestReadResult)
	done := make(chan struct{})
	defer close(done)
	go readRequests(reader, requests, done)
	for {
		select {
		case result, ok := <-requests:
			if !ok {
				return nil
			}
			if result.err != nil {
				if result.err == io.EOF {
					return nil
				}
				return result.err
			}
			for _, message := range handler.Handle(result.request) {
				if err := Write(w, message); err != nil {
					return err
				}
				if event, ok := message.(Event); ok && event.Event == "terminated" {
					return nil
				}
			}
		case event := <-handler.Events():
			if err := Write(w, event); err != nil {
				return err
			}
			if event.Event == "terminated" {
				return nil
			}
		}
	}
}
