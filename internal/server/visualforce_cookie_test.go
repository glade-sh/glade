package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

// The Apex Cookie decides Secure and HttpOnly; the route must not add or drop
// either flag. These cases are the sink that CodeQL's cookie queries cannot
// analyze in bounded time.
func TestVisualforceApexCookiesCarryExactSecurityFlags(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cookie vm.VisualforceResponseCookie
		want   []string
		absent []string
	}{
		{"secure and httponly", vm.VisualforceResponseCookie{Name: "a", Value: "1", Path: "/", MaxAge: 60, Secure: true, HTTPOnly: true},
			[]string{"apex__a=1", "Path=/", "Max-Age=60", "Secure", "HttpOnly"}, nil},
		{"neither flag", vm.VisualforceResponseCookie{Name: "b", Value: "2", Path: "/", MaxAge: 60},
			[]string{"apex__b=2", "Max-Age=60"}, []string{"Secure", "HttpOnly"}},
		{"secure only", vm.VisualforceResponseCookie{Name: "c", Value: "3", Path: "/", MaxAge: 60, Secure: true},
			[]string{"apex__c=3", "Secure"}, []string{"HttpOnly"}},
		{"httponly only", vm.VisualforceResponseCookie{Name: "d", Value: "4", Path: "/", MaxAge: 60, HTTPOnly: true},
			[]string{"apex__d=4", "HttpOnly"}, []string{"Secure"}},
		{"zero max age expires", vm.VisualforceResponseCookie{Name: "e", Value: "5", Path: "/", MaxAge: 0, Secure: true, HTTPOnly: true},
			[]string{"apex__e=5", "Max-Age=0", "Secure", "HttpOnly"}, nil},
		{"unset max age", vm.VisualforceResponseCookie{Name: "f", Value: "6", Path: "/", MaxAge: -1},
			[]string{"apex__f=6"}, []string{"Max-Age", "Secure", "HttpOnly"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			applyVisualforceRequestResponse(rec, visualforce.PageRenderResult{Cookies: []vm.VisualforceResponseCookie{tc.cookie}})
			header := rec.Result().Header.Values("Set-Cookie")
			if len(header) != 1 {
				t.Fatalf("Set-Cookie = %q, want one cookie", header)
			}
			if !strings.HasPrefix(header[0], "apex__"+tc.cookie.Name+"=") {
				t.Errorf("cookie %q lacks the apex__ prefix", header[0])
			}
			parts := strings.Split(header[0], "; ")
			for _, want := range tc.want {
				if !contains(parts, want) {
					t.Errorf("Set-Cookie %q lacks %q", header[0], want)
				}
			}
			for _, absent := range tc.absent {
				for _, part := range parts {
					if part == absent || strings.HasPrefix(part, absent+"=") {
						t.Errorf("Set-Cookie %q must not carry %q", header[0], absent)
					}
				}
			}
		})
	}
}

func contains(parts []string, want string) bool {
	for _, part := range parts {
		if part == want {
			return true
		}
	}
	return false
}
