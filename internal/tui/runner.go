package tui

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/cliui"
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

type RunUpdate struct {
	Args   []string
	Event  *cliui.Event
	Stderr string
	Result *RunResult
	Err    error
}

type StreamingRunner interface {
	RunStreaming(context.Context, []string) (<-chan RunUpdate, error)
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

func (r ExecRunner) RunStreaming(ctx context.Context, args []string) (<-chan RunUpdate, error) {
	bin := r.Bin
	if bin == "" {
		bin = os.Args[0]
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = r.Dir
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	updates := make(chan RunUpdate, 128)
	go func() {
		defer close(updates)
		runArgs := append([]string{}, args...)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(&stdout, stdoutPipe)
		}()
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				line := scanner.Text()
				stderr.WriteString(line)
				stderr.WriteByte('\n')
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if event, ok := parseProgressEventLine(trimmed); ok {
					event := event
					updates <- RunUpdate{Args: runArgs, Event: &event}
					continue
				}
				updates <- RunUpdate{Args: runArgs, Stderr: trimmed}
			}
		}()
		err := cmd.Wait()
		wg.Wait()
		code := 0
		if err != nil {
			code = 1
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			}
		}
		result := RunResult{Args: runArgs, ExitCode: code, Stdout: stdout.String(), Stderr: stderr.String()}
		updates <- RunUpdate{Args: runArgs, Result: &result, Err: err}
	}()
	return updates, nil
}
