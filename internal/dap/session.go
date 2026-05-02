package dap

import (
	"io"
)

func Serve(r io.Reader, w io.Writer, handler *Handler) error {
	reader := NewReader(r)
	for {
		request, err := reader.ReadRequest()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		for _, message := range handler.Handle(request) {
			if err := Write(w, message); err != nil {
				return err
			}
			if event, ok := message.(Event); ok && event.Event == "terminated" {
				return nil
			}
		}
	}
}
