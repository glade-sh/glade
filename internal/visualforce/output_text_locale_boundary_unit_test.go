package visualforce

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Named styles have native coverage only in the en_US profile. These checks
// preserve the local unsupported boundary; they do not assert German/French output.
func TestOutputTextDateStyleLocaleBoundaryUnitRoute(t *testing.T) {
	for _, locale := range []string{"en_US", "de_DE", "fr_FR"} {
		for _, kind := range []string{"date", "time"} {
			for _, style := range []string{"", "short", "medium", "long", "full"} {
				t.Run(locale+"/"+kind+"/"+style, func(t *testing.T) {
					machine := vm.New(nil)
					user := storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(locale)}}
					machine.SetOrg(&storage.OrgState{Objects: map[string]storage.ObjectState{"User": {Records: map[storage.ID]storage.Record{user.ID: user}}}})
					machine.SetCurrentUser(user)
					stamp := vm.Object("Datetime")
					stamp.Fields["value"] = vm.String("2024-02-09T17:06:03Z")
					got, err := formatOutputTextArgument(stamp, []string{kind, style}, &RenderContext{VM: machine})
					if locale == "en_US" {
						if err != nil || got == "" {
							t.Fatalf("captured English style: got=%q error=%v", got, err)
						}
					} else {
						var runtime *vm.RuntimeError
						if !errors.As(err, &runtime) || runtime.Type != "UnsupportedFeature" {
							t.Fatalf("got=%q error=%v, want UnsupportedFeature locale boundary", got, err)
						}
					}
				})
			}
		}
	}
}
