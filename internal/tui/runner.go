package tui

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

type RunResult struct {
	Args     []string
	ExitCode int
	Stdout   string
	Stderr   string
}

type Runner interface {
	Run(context.Context, []string) (RunResult, error)
}

type ExecRunner struct {
	Bin string
	Dir string
}

func (r ExecRunner) Run(ctx context.Context, args []string) (RunResult, error) {
	bin := r.Bin
	if bin == "" {
		bin = os.Args[0]
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = r.Dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		code = 1
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		}
	}
	return RunResult{Args: append([]string{}, args...), ExitCode: code, Stdout: stdout.String(), Stderr: stderr.String()}, err
}
