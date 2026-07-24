package testdaemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

const (
	OpPing        = "ping"
	OpPong        = "pong"
	OpRun         = "run"
	OpRunResult   = "run_result"
	OpShutdown    = "shutdown"
	OpShutdownAck = "shutdown_ack"
	OpError       = "error"
)

const (
	ProtocolVersionV1     = 1
	MaxProtocolFrameBytes = 8 << 20
)

type Request struct {
	Op           string `json:"op"`
	ID           string `json:"id,omitempty"`
	Filter       string `json:"filter,omitempty"`
	ChangedSince string `json:"changedSince,omitempty"`
}

type Response struct {
	Op        string               `json:"op"`
	ID        string               `json:"id,omitempty"`
	OK        bool                 `json:"ok"`
	Error     string               `json:"error,omitempty"`
	Ready     bool                 `json:"ready,omitempty"`
	Warming   bool                 `json:"warming,omitempty"`
	Project   string               `json:"project,omitempty"`
	Run       *testreport.Run      `json:"run,omitempty"`
	Selection *watch.TestSelection `json:"selection,omitempty"`
}

type RequestV1 struct {
	Version int           `json:"version"`
	Op      string        `json:"op"`
	ID      string        `json:"id,omitempty"`
	Run     *RunRequestV1 `json:"run,omitempty"`
}

type RunRequestV1 struct {
	Filter              string           `json:"filter"`
	ChangedSince        string           `json:"changedSince"`
	SelectedClasses     []string         `json:"selectedClasses"`
	SelectedMethod      string           `json:"selectedMethod"`
	LimitMode           string           `json:"limitMode"`
	LimitCaps           LimitCapsV1      `json:"limitCaps"`
	LimitCapsSet        bool             `json:"limitCapsSet"`
	TraceBlocked        bool             `json:"traceBlocked"`
	TraceAll            bool             `json:"traceAll"`
	SlowTestThresholdMS int64            `json:"slowTestThresholdMs"`
	TimeoutMS           int64            `json:"timeoutMs"`
	Parallelism         int              `json:"parallelism"`
	ParallelMethods     bool             `json:"parallelMethods"`
	NoDiskCache         bool             `json:"noDiskCache"`
	ClassDurationMS     map[string]int64 `json:"classDurationMs"`
	MethodDurationMS    map[string]int64 `json:"methodDurationMs"`
	PerfCounters        bool             `json:"perfCounters"`
	ShardCount          int              `json:"shardCount"`
	ShardIndex          int              `json:"shardIndex"`
	ReturnClassShards   bool             `json:"returnClassShards"`
}

type LimitCapsV1 struct {
	Queries             int `json:"queries"`
	QueryRows           int `json:"queryRows"`
	DMLStatements       int `json:"dmlStatements"`
	DMLRows             int `json:"dmlRows"`
	HeapSize            int `json:"heapSize"`
	CPUTimeMS           int `json:"cpuTimeMs"`
	Callouts            int `json:"callouts"`
	AsyncJobs           int `json:"asyncJobs"`
	FutureCalls         int `json:"futureCalls"`
	QueueableJobs       int `json:"queueableJobs"`
	BatchJobs           int `json:"batchJobs"`
	ScheduledJobs       int `json:"scheduledJobs"`
	EmailInvokes        int `json:"emailInvocations"`
	SOSLQueries         int `json:"soslQueries"`
	QueryLocatorRows    int `json:"queryLocatorRows"`
	RunAs               int `json:"runAs"`
	Savepoints          int `json:"savepoints"`
	SavepointRollbacks  int `json:"savepointRollbacks"`
	PublishImmediateDML int `json:"publishImmediateDml"`
}

type ResponseV1 struct {
	Version   int                  `json:"version"`
	Op        string               `json:"op"`
	ID        string               `json:"id,omitempty"`
	OK        bool                 `json:"ok"`
	Error     string               `json:"error,omitempty"`
	Ready     bool                 `json:"ready"`
	Warming   bool                 `json:"warming"`
	Project   string               `json:"project,omitempty"`
	Run       *testreport.Run      `json:"run,omitempty"`
	Selection *watch.TestSelection `json:"selection,omitempty"`
	ShardPlan *ClassShardPlanV1    `json:"shardPlan,omitempty"`
}

type ClassShardPlanV1 struct {
	Shards []ClassShardV1 `json:"shards"`
}

type ClassShardV1 struct {
	Index           int      `json:"index"`
	TotalDurationMS int64    `json:"totalDurationMs"`
	Classes         []string `json:"classes"`
}

// testReportRunV1Wire includes the derived summary emitted by
// testreport.Run.MarshalJSON. Decoding through this explicit shape keeps the
// response decoder strict without rejecting output produced by its encoder.
type testReportRunV1Wire struct {
	Name         string                   `json:"name,omitempty"`
	DurationMS   int64                    `json:"durationMs,omitempty"`
	Dependencies []typesys.DependencyInfo `json:"dependencies,omitempty"`
	Summary      testreport.Summary       `json:"summary"`
	Suites       []testreport.Suite       `json:"suites"`
}

type responseV1Wire struct {
	Version   int                  `json:"version"`
	Op        string               `json:"op"`
	ID        string               `json:"id,omitempty"`
	OK        bool                 `json:"ok"`
	Error     string               `json:"error,omitempty"`
	Ready     bool                 `json:"ready"`
	Warming   bool                 `json:"warming"`
	Project   string               `json:"project,omitempty"`
	Run       *testReportRunV1Wire `json:"run,omitempty"`
	Selection *watch.TestSelection `json:"selection,omitempty"`
	ShardPlan *ClassShardPlanV1    `json:"shardPlan,omitempty"`
}

func EncodeRequestV1(w io.Writer, request RequestV1) error {
	if err := validateProtocolVersionV1(request.Version); err != nil {
		return err
	}
	return encodeProtocolV1Frame(w, request)
}

func DecodeRequestV1(r io.Reader) (RequestV1, error) {
	var request RequestV1
	if err := decodeProtocolV1Frame(r, &request); err != nil {
		return RequestV1{}, err
	}
	if err := validateProtocolVersionV1(request.Version); err != nil {
		return RequestV1{}, err
	}
	return request, nil
}

func EncodeResponseV1(w io.Writer, response ResponseV1) error {
	if err := validateProtocolVersionV1(response.Version); err != nil {
		return err
	}
	return encodeProtocolV1Frame(w, response)
}

func DecodeResponseV1(r io.Reader) (ResponseV1, error) {
	var wire responseV1Wire
	if err := decodeProtocolV1Frame(r, &wire); err != nil {
		return ResponseV1{}, err
	}
	if err := validateProtocolVersionV1(wire.Version); err != nil {
		return ResponseV1{}, err
	}
	response := ResponseV1{
		Version:   wire.Version,
		Op:        wire.Op,
		ID:        wire.ID,
		OK:        wire.OK,
		Error:     wire.Error,
		Ready:     wire.Ready,
		Warming:   wire.Warming,
		Project:   wire.Project,
		Selection: wire.Selection,
		ShardPlan: wire.ShardPlan,
	}
	if wire.Run != nil {
		response.Run = &testreport.Run{
			Name:         wire.Run.Name,
			DurationMS:   wire.Run.DurationMS,
			Dependencies: wire.Run.Dependencies,
			Suites:       wire.Run.Suites,
		}
		if reconstructed := response.Run.Summary(); wire.Run.Summary != reconstructed {
			return ResponseV1{}, fmt.Errorf("decode test daemon protocol frame: run summary does not match suites")
		}
	}
	return response, nil
}

func validateProtocolVersionV1(version int) error {
	if version != ProtocolVersionV1 {
		return fmt.Errorf("unsupported test daemon protocol version %d; supported version is %d", version, ProtocolVersionV1)
	}
	return nil
}

func encodeProtocolV1Frame(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode test daemon protocol frame: %w", err)
	}
	if len(data) > MaxProtocolFrameBytes {
		return fmt.Errorf("test daemon protocol frame is %d bytes; maximum is %d", len(data), MaxProtocolFrameBytes)
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	if err != nil {
		return fmt.Errorf("write test daemon protocol frame: %w", err)
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func decodeProtocolV1Frame(r io.Reader, value any) error {
	data, err := readProtocolV1Frame(r)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode test daemon protocol frame: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode test daemon protocol frame: multiple JSON values")
		}
		return fmt.Errorf("decode test daemon protocol frame trailer: %w", err)
	}
	return nil
}

func readProtocolV1Frame(r io.Reader) ([]byte, error) {
	reader, ok := r.(io.ByteReader)
	if !ok {
		return nil, fmt.Errorf("test daemon protocol decoder requires a persistent io.ByteReader such as *bufio.Reader")
	}
	data := make([]byte, 0, 4096)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("test daemon protocol frame is not LF terminated")
			}
			return nil, fmt.Errorf("read test daemon protocol frame: %w", err)
		}
		if b == '\n' {
			if len(data) > 0 && data[len(data)-1] == '\r' {
				data = data[:len(data)-1]
			}
			if len(data) == 0 {
				return nil, fmt.Errorf("test daemon protocol frame is empty")
			}
			if len(data) > MaxProtocolFrameBytes {
				return nil, fmt.Errorf("test daemon protocol frame exceeds %d bytes", MaxProtocolFrameBytes)
			}
			return data, nil
		}

		data = append(data, b)
		if len(data) > MaxProtocolFrameBytes {
			if len(data) == MaxProtocolFrameBytes+1 && b == '\r' {
				continue
			}
			return nil, fmt.Errorf("test daemon protocol frame exceeds %d bytes", MaxProtocolFrameBytes)
		}
	}
}
