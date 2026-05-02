package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const contentLengthHeader = "Content-Length"

type ProtocolReader struct {
	r *bufio.Reader
}

func NewProtocolReader(r io.Reader) *ProtocolReader {
	return &ProtocolReader{r: bufio.NewReader(r)}
}

func (r *ProtocolReader) Read() ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("malformed LSP header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), contentLengthHeader) {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid LSP Content-Length %q", strings.TrimSpace(value))
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing LSP Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r.r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func WriteMessage(w io.Writer, message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s: %d\r\n\r\n", contentLengthHeader, len(data)); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func Serve(r io.Reader, w io.Writer, handler *Handler) error {
	reader := NewProtocolReader(r)
	for {
		data, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		response, err := handler.HandleJSON(data)
		if err != nil {
			return err
		}
		if len(response) > 0 {
			if err := WriteMessage(w, json.RawMessage(response)); err != nil {
				return err
			}
		}
		if handler.Shutdown() {
			return nil
		}
	}
}
