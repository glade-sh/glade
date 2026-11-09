package gladecli

import "testing"

func TestV035PublicDemoOptions(t *testing.T) {
	l, err := parseDevLWCOptions([]string{"--allow-public-demo"})
	if err != nil || !l.publicDemo {
		t.Fatalf("lwc option: %v", err)
	}
	d, err := parseDBUIOptions([]string{"--allow-public-demo"})
	if err != nil || !d.publicDemo {
		t.Fatalf("db option: %v", err)
	}
}
