package gladecli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelpContractRoutes(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"help", "toolchain"}, []string{"auto-discovered", "GLADE_HOME selects", "executable ancestors"}},
		{[]string{"help", "dev"}, []string{"substring", "ignored in watch mode"}},
		{[]string{"help", "debug"}, []string{"always writes parsed-log JSON", "compatibility no-op"}},
		{[]string{"test", "serve", "--help"}, []string{"--socket <path>", "--no-watch", "--no-warm", "Clients discover only"}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := Run(context.Background(), tc.args, &out, &errOut); code != 0 {
				t.Fatalf("exit %d: %s", code, errOut.String())
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q", want)
				}
			}
		})
	}
}
