package playground

import (
	"time"

	"github.com/open-aer/oaer/internal/trace"
	"github.com/open-aer/oaer/internal/vm"
)

type WorkspaceOptions struct {
	DataRoot    string
	ID          string
	ProjectRoot string
}

type ExampleProject struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	FileCount   int      `json:"fileCount"`
	Source      string   `json:"source,omitempty"`
	Path        string   `json:"path,omitempty"`
}

type ProjectReference struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type WorkspaceMetadata struct {
	ID            string          `json:"id"`
	Root          string          `json:"root"`
	ProjectRoot   string          `json:"projectRoot"`
	ExampleID     string          `json:"exampleId,omitempty"`
	Files         []WorkspaceFile `json:"files"`
	AnonymousBody string          `json:"anonymousBody,omitempty"`
	WorkspaceHash string          `json:"workspaceHash,omitempty"`
	LimitMode     vm.LimitMode    `json:"limitMode,omitempty"`
}

type WorkspaceFile struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Version int    `json:"version"`
	Size    int64  `json:"size"`
}

type FileSaveRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Version int    `json:"version"`
}

type FileSaveResponse struct {
	File          WorkspaceFile `json:"file"`
	WorkspaceHash string        `json:"workspaceHash"`
}

type RunMode string

const (
	RunModeScratch RunMode = "scratch"
	RunModePersist RunMode = "persist"
)

type RunStatus string

const (
	RunStatusPass          RunStatus = "pass"
	RunStatusCompileError  RunStatus = "compile_error"
	RunStatusRuntimeError  RunStatus = "runtime_error"
	RunStatusInternalError RunStatus = "internal_error"
)

type RunRequest struct {
	AnonymousBody string       `json:"anonymousBody"`
	Mode          RunMode      `json:"mode"`
	LimitMode     vm.LimitMode `json:"limitMode"`
	UseCache      bool         `json:"useCache"`
}

type RunResult struct {
	RunID        string        `json:"runId"`
	CacheHit     bool          `json:"cacheHit"`
	Status       RunStatus     `json:"status"`
	CompileMS    int64         `json:"compileMs"`
	ExecuteMS    int64         `json:"executeMs"`
	Diagnostics  []Diagnostic  `json:"diagnostics,omitempty"`
	Logs         []string      `json:"logs,omitempty"`
	Vars         []VarResult   `json:"vars,omitempty"`
	Limits       vm.Limits     `json:"limits,omitempty"`
	LimitMode    vm.LimitMode  `json:"limitMode,omitempty"`
	Trace        []trace.Event `json:"trace,omitempty"`
	OrgDiff      []OrgDiff     `json:"orgDiff,omitempty"`
	CacheKey     string        `json:"cacheKey,omitempty"`
	StartedAt    time.Time     `json:"startedAt,omitempty"`
	CompletedAt  time.Time     `json:"completedAt,omitempty"`
	ErrorMessage string        `json:"errorMessage,omitempty"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

type VarResult struct {
	Name  string   `json:"name"`
	Type  string   `json:"type,omitempty"`
	Value vm.Value `json:"value"`
}

type OrgDiff struct {
	Object      string   `json:"object"`
	Inserted    int      `json:"inserted"`
	Updated     int      `json:"updated"`
	Deleted     int      `json:"deleted"`
	InsertedIDs []string `json:"insertedIds,omitempty"`
	UpdatedIDs  []string `json:"updatedIds,omitempty"`
	DeletedIDs  []string `json:"deletedIds,omitempty"`
}

type RunnerOptions struct {
	Version string
	Org     *vmOrgStore
	DBPath  string
}

type ServerOptions struct {
	Version           string
	DBPath            string
	DefaultLimitMode  vm.LimitMode
	ProjectReferences []ProjectReference
}
