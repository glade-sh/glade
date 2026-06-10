package dap

import (
	"io"
)

type requestReadResult struct {
	request Request
	err     error
}

func Serve(r io.Reader, w io.Writer, handler *Handler) error {
	reader := NewReader(r)
	requests := make(chan requestReadResult)
	go func() {
		defer close(requests)
		for {
			request, err := reader.ReadRequest()
			requests <- requestReadResult{request: request, err: err}
			if err != nil {
				return
			}
		}
	}()
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
