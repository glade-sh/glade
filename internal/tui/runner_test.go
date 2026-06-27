package tui

import (
	"context"
	"testing"
)

type recordingRunner struct {
	result RunResult
	args   []string
}

func (r *recordingRunner) Run(_ context.Context, args []string) (RunResult, error) {
	r.args = append([]string{}, args...)
	r.result.Args = append([]string{}, args...)
	return r.result, nil
}

func TestRecordingRunnerCapturesArgs(t *testing.T) {
	runner := &recordingRunner{result: RunResult{ExitCode: 0, Stdout: "{}"}}
	result, err := runner.Run(context.Background(), []string{"doctor", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || runner.args[0] != "doctor" {
		t.Fatalf("result=%#v args=%#v", result, runner.args)
	}
}
