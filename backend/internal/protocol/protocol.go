// Package protocol mirrors the runner protocol v1 (runners/protocol/*.schema.json).
package protocol

// Version is the protocol version this worker speaks.
const Version = 1

// Request is the event sent to a runner. Expected outputs are never included.
type Request struct {
	ProtocolVersion int        `json:"protocol_version"`
	SubmissionID    string     `json:"submission_id"`
	Language        string     `json:"language"`
	Source          string     `json:"source"`
	Limits          Limits     `json:"limits"`
	TestCases       []TestCase `json:"test_cases"`
	StopOnFailure   bool       `json:"stop_on_failure"`
}

// Limits apply to every test case.
type Limits struct {
	TimeMS        int `json:"time_ms"`
	MemoryMB      int `json:"memory_mb"`
	OutputKB      int `json:"output_kb,omitempty"`
	CompileTimeMS int `json:"compile_time_ms,omitempty"`
}

// TestCase is one input for the program.
type TestCase struct {
	ID    string `json:"id"`
	Input string `json:"input"`
}

// Response is what the runner returns.
type Response struct {
	ProtocolVersion int          `json:"protocol_version"`
	SubmissionID    string       `json:"submission_id"`
	Compile         Compile      `json:"compile"`
	Results         []CaseResult `json:"results"`
	Truncated       bool         `json:"truncated"`
}

// Compile is the outcome of the compile (or syntax check) step.
type Compile struct {
	Status CompileStatus `json:"status"`
	Output string        `json:"output"`
}

// CaseResult describes how the program ran on one test case.
type CaseResult struct {
	ID         string `json:"id"`
	Status     Status `json:"status"`
	CPUTimeMS  int    `json:"cpu_time_ms"`
	WallTimeMS int    `json:"wall_time_ms"`
	MemoryKB   int    `json:"memory_kb"`
	ExitCode   *int   `json:"exit_code"`
	Signal     *int   `json:"signal"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
}

// CompileStatus values.
type CompileStatus string

// Compile statuses.
const (
	CompileOK        CompileStatus = "ok"
	CompileError     CompileStatus = "error"
	CompileTimeLimit CompileStatus = "time_limit"
)

// Status of one run.
type Status string

// Run statuses.
const (
	StatusOK           Status = "ok"
	StatusTimeLimit    Status = "time_limit"
	StatusMemoryLimit  Status = "memory_limit"
	StatusOutputLimit  Status = "output_limit"
	StatusRuntimeError Status = "runtime_error"
	StatusSkipped      Status = "skipped"
)
