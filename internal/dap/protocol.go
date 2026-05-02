package dap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const contentLengthHeader = "Content-Length"

type Reader struct {
	r *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReader(r)}
}

func (r *Reader) Read() (json.RawMessage, error) {
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
			return nil, fmt.Errorf("malformed DAP header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), contentLengthHeader) {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid DAP Content-Length %q", strings.TrimSpace(value))
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing DAP Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r.r, body); err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

func (r *Reader) ReadRequest() (Request, error) {
	raw, err := r.Read()
	if err != nil {
		return Request{}, err
	}
	return DecodeRequest(raw)
}

func DecodeRequest(raw []byte) (Request, error) {
	var request Request
	if err := json.Unmarshal(raw, &request); err != nil {
		return Request{}, err
	}
	if request.Type != MessageTypeRequest {
		return Request{}, fmt.Errorf("expected DAP request, got %q", request.Type)
	}
	if request.Command == "" {
		return Request{}, fmt.Errorf("DAP request is missing command")
	}
	return request, nil
}

func Write(w io.Writer, message any) error {
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

func Encode(message any) ([]byte, error) {
	var out bytes.Buffer
	if err := Write(&out, message); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
