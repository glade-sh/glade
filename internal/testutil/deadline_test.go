package testutil

import (
	"testing"
	"time"
)

func TestDeadlineScalesAndNeverShortens(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want time.Duration
	}{
		{"", 5 * time.Second},
		{"1", 5 * time.Second},
		{"3", 15 * time.Second},
		{"2.5", 12500 * time.Millisecond},
		{"0.1", 5 * time.Second},
		{"-2", 5 * time.Second},
		{"junk", 5 * time.Second},
	} {
		t.Setenv(TimeScaleEnv, tc.env)
		if got := Deadline(5 * time.Second); got != tc.want {
			t.Errorf("%s=%q: Deadline = %s, want %s", TimeScaleEnv, tc.env, got, tc.want)
		}
	}
}
