package main

import (
	"runtime/debug"
	"testing"
)

func TestTuneExecGC(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		gogc string
		want int
	}{
		{name: "exec", args: []string{"exec"}, want: 400},
		{name: "explicit", args: []string{"exec"}, gogc: "73", want: 73},
		{name: "disabled", args: []string{"exec"}, gogc: "off", want: 73},
		{name: "other command", args: []string{"test"}, want: 73},
		{name: "no command", want: 73},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GOGC", test.gogc)
			old := debug.SetGCPercent(73)
			defer debug.SetGCPercent(old)
			tuneExecGC(test.args)
			if got := debug.SetGCPercent(73); got != test.want {
				t.Fatalf("GC percent = %d, want %d", got, test.want)
			}
		})
	}
}

func TestRootContextStopCancelsContext(t *testing.T) {
	ctx, stop := rootContext()
	stop()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("root context was not canceled by stop")
	}
}
