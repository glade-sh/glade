// Package dataweave hosts the official DataWeave engine in a bounded Java process.
// It does not depend on Apex VM types or install runtime dependencies implicitly.
package dataweave

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed Adapter.java
var adapterSource []byte

//go:embed OwnedApexFormat.java
var apexFormatSource []byte

const apexFormatServicePath = "META-INF/services/org.mule.weave.v2.module.DataFormat"
const apexFormatService = "OwnedApexFormat\nOwnedApexFormat$JavaInputFormat\n"

const maxFieldBytes = 16 << 20

// Input is an encoded value. An empty MIMEType uses the unchanged DWL input declaration. JSON numbers remain
// decimal text; callers must not first round them through float64.
type Input struct {
	MIMEType string
	Data     []byte
	Typed    *TypedValue
}

// Module is an explicitly selected project resource. The import binding is the
// Request.Modules key; original metadata is retained in the request identity.
type Module struct {
	Name       string
	Namespace  string
	APIVersion string
	Source     string
}
type Request struct {
	APIVersion string
	Name       string
	Source     string
	Inputs     map[string]Input
	Modules    map[string]Module
}
type Result struct {
	RequestSHA256 string
	MIMEType      string
	Charset       string
	Data          []byte
	Typed         *TypedValue
}

// EngineError retains the engine's error type and message. The Apex host owns
// compatibility exception mapping and must distinguish local failures from parity.
type EngineError struct {
	Phase   string
	Type    string
	Message string
}

func (e *EngineError) Error() string { return e.Type + ": " + e.Message }

// HostError identifies local execution failures separately from engine diagnostics.
type HostError struct {
	Kind   string
	Detail string
	Cause  error
}

func (e *HostError) Error() string {
	if e.Cause != nil {
		return e.Detail + ": " + e.Cause.Error()
	}
	return e.Detail
}
func (e *HostError) Unwrap() error { return e.Cause }

// Runtime refers to an explicitly provisioned adapter and locked engine jars.
// Timeout bounds compilation as well as execution. Zero uses 15 seconds.
type Runtime struct {
	JavaPath         string
	ClassPath        []string
	AdapterDirectory string
	Timeout          time.Duration
}

func (r Runtime) Execute(ctx context.Context, request Request) (Result, error) {
	if r.JavaPath == "" || r.AdapterDirectory == "" || len(r.ClassPath) == 0 {
		return Result{}, &HostError{Kind: "toolchain", Detail: "DataWeave Java toolchain is not configured"}
	}
	if err := verifyClassPath(r.ClassPath); err != nil {
		return Result{}, &HostError{Kind: "toolchain", Detail: "DataWeave classpath verification failed", Cause: err}
	}
	payload, err := encodeRequest(request)
	if err != nil {
		return Result{}, &HostError{Kind: "request", Detail: "invalid DataWeave request", Cause: err}
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	classPath := append(append([]string(nil), r.ClassPath...), r.AdapterDirectory)
	cmd := exec.CommandContext(ctx, r.JavaPath, "-Xmx256m", "-Dfile.encoding=UTF-8", "-cp", strings.Join(classPath, string(os.PathListSeparator)), "Adapter")
	cmd.Env = isolatedEnvironment()
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr boundedBuffer
	stdout.limit = maxFieldBytes + (1 << 20)
	stderr.limit = 64 << 10
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		kind := "cancelled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			kind = "timeout"
		}
		return Result{}, &HostError{Kind: kind, Detail: "DataWeave process stopped", Cause: ctx.Err()}
	}
	if stdout.exceeded || stderr.exceeded {
		return Result{}, &HostError{Kind: "output-limit", Detail: "DataWeave process output exceeds byte limit"}
	}
	if err != nil {
		return Result{}, &HostError{Kind: "process", Detail: "DataWeave process failed: " + stderr.String(), Cause: err}
	}
	result, err := decodeResult(stdout.Bytes())
	var engineError *EngineError
	var hostError *HostError
	if err != nil && !errors.As(err, &engineError) && !errors.As(err, &hostError) {
		err = &HostError{Kind: "protocol", Detail: "invalid DataWeave process response", Cause: err}
	}
	digest := sha256.Sum256(payload)
	result.RequestSHA256 = hex.EncodeToString(digest[:])
	return result, err
}

// CompileAdapter builds the owned Java host against a separately verified engine
// classpath. Provisioners should publish the completed directory atomically.
func CompileAdapter(ctx context.Context, javac string, classPath []string, outputDir string) error {
	if javac == "" || len(classPath) == 0 {
		return errors.New("DataWeave Java compiler is not configured")
	}
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return err
	}
	source := filepath.Join(outputDir, "Adapter.java")
	if err := os.WriteFile(source, adapterSource, 0600); err != nil {
		return err
	}
	formatSource := filepath.Join(outputDir, "OwnedApexFormat.java")
	if err := os.WriteFile(formatSource, apexFormatSource, 0600); err != nil {
		return err
	}
	descriptor := filepath.Join(outputDir, filepath.FromSlash(apexFormatServicePath))
	if err := os.MkdirAll(filepath.Dir(descriptor), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(descriptor, []byte(apexFormatService), 0600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, javac, "--release", "17", "-cp", strings.Join(classPath, string(os.PathListSeparator)), "-d", outputDir, source, formatSource)
	cmd.Env = isolatedEnvironment()
	var output boundedBuffer
	output.limit = 64 << 10
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compile DataWeave adapter: %w: %s", err, output.String())
	}
	if output.exceeded {
		return errors.New("DataWeave compiler output exceeds byte limit")
	}
	return nil
}

func isolatedEnvironment() []string {
	// Java launcher options, classpaths, credentials and user home settings are not
	// inherited. Windows requires SystemRoot to load the platform Java libraries.
	env := []string{"LANG=C.UTF-8"}
	if root := os.Getenv("SystemRoot"); root != "" {
		env = append(env, "SystemRoot="+root)
	}
	return env
}

func encodeRequest(request Request) ([]byte, error) {
	for _, text := range []string{request.Name, request.Source, request.APIVersion} {
		if !utf8.ValidString(text) {
			return nil, errors.New("DataWeave source metadata must be valid UTF-8")
		}
	}
	if len(request.Inputs) > 1024 {
		return nil, errors.New("too many DataWeave inputs")
	}
	var out bytes.Buffer
	binary.Write(&out, binary.BigEndian, uint32(5))
	for _, s := range []string{request.Name, request.Source, request.APIVersion} {
		if err := writeField(&out, []byte(s)); err != nil {
			return nil, err
		}
	}
	binary.Write(&out, binary.BigEndian, uint32(len(request.Inputs)))
	names := make([]string, 0, len(request.Inputs))
	for name := range request.Inputs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := request.Inputs[name]
		if name == "" || !utf8.ValidString(name) || !utf8.ValidString(value.MIMEType) {
			return nil, errors.New("DataWeave input requires a valid UTF-8 name and MIME type")
		}
		data := value.Data
		mode := byte(0)
		if value.Typed != nil {
			if len(value.Data) > 0 || value.MIMEType != "" {
				return nil, errors.New("typed DataWeave input cannot also specify raw bytes or MIME type")
			}
			var err error
			data, err = encodeTyped(*value.Typed)
			if err != nil {
				return nil, err
			}
			mode = 1
		}
		for _, field := range [][]byte{[]byte(name), []byte(value.MIMEType)} {
			if err := writeField(&out, field); err != nil {
				return nil, err
			}
		}
		out.WriteByte(mode)
		for _, field := range [][]byte{data} {
			if err := writeField(&out, field); err != nil {
				return nil, err
			}
		}
		if out.Len() > maxFieldBytes {
			return nil, errors.New("DataWeave request exceeds byte limit")
		}
	}
	if len(request.Modules) > 1024 {
		return nil, errors.New("too many DataWeave project modules")
	}
	binary.Write(&out, binary.BigEndian, uint32(len(request.Modules)))
	moduleNames := make([]string, 0, len(request.Modules))
	for name := range request.Modules {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)
	for _, name := range moduleNames {
		module := request.Modules[name]
		if name == "" || module.Name == "" {
			return nil, errors.New("DataWeave module requires an import binding and resource name")
		}
		for _, field := range []string{name, module.Name, module.Namespace, module.APIVersion, module.Source} {
			if !utf8.ValidString(field) {
				return nil, errors.New("DataWeave module source metadata must be valid UTF-8")
			}
			if err := writeField(&out, []byte(field)); err != nil {
				return nil, err
			}
		}
		if out.Len() > maxFieldBytes {
			return nil, errors.New("DataWeave request exceeds byte limit")
		}
	}
	if out.Len() > maxFieldBytes {
		return nil, errors.New("DataWeave request exceeds byte limit")
	}
	return out.Bytes(), nil
}
func writeField(out io.Writer, value []byte) error {
	if len(value) > maxFieldBytes {
		return errors.New("DataWeave field exceeds byte limit")
	}
	if err := binary.Write(out, binary.BigEndian, uint32(len(value))); err != nil {
		return err
	}
	_, err := out.Write(value)
	return err
}
func readField(in *bytes.Reader) ([]byte, error) {
	var size uint32
	if err := binary.Read(in, binary.BigEndian, &size); err != nil {
		return nil, err
	}
	if size > maxFieldBytes || uint64(size) > uint64(in.Len()) {
		return nil, errors.New("invalid DataWeave response field length")
	}
	value := make([]byte, size)
	_, err := io.ReadFull(in, value)
	return value, err
}
func decodeResult(data []byte) (Result, error) {
	in := bytes.NewReader(data)
	status, err := in.ReadByte()
	if err != nil {
		return Result{}, err
	}
	if status > 2 {
		return Result{}, errors.New("invalid DataWeave response status")
	}
	first, err := readField(in)
	if err != nil {
		return Result{}, err
	}
	second, err := readField(in)
	if err != nil {
		return Result{}, err
	}
	if status == 2 {
		if in.Len() != 0 {
			return Result{}, errors.New("trailing DataWeave host error bytes")
		}
		return Result{}, &HostError{Kind: string(first), Detail: string(second)}
	}
	if status == 1 {
		message, err := readField(in)
		if err != nil {
			return Result{}, err
		}
		if in.Len() != 0 {
			return Result{}, errors.New("trailing DataWeave error bytes")
		}
		return Result{}, &EngineError{Phase: string(first), Type: string(second), Message: string(message)}
	}
	value, err := readField(in)
	if err != nil {
		return Result{}, err
	}
	if in.Len() != 0 {
		return Result{}, errors.New("trailing DataWeave response bytes")
	}
	result := Result{MIMEType: string(first), Charset: string(second), Data: value}
	if result.MIMEType == "application/apex" {
		typed, err := decodeTyped(value)
		if err != nil {
			return Result{}, &HostError{Kind: "typed-output", Detail: "invalid DataWeave typed output", Cause: err}
		}
		result.Typed = &typed
		result.Data = nil
	}
	return result, nil
}

type boundedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.Len()
	if len(p) > remaining {
		if remaining > 0 {
			b.Buffer.Write(p[:remaining])
		}
		b.exceeded = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
