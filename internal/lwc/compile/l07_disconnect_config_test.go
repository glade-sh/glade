package compile_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
)

// Replay only the disconnect/config row without running the other L07 rows.
// Its full native answer, including lifecycle counters and both wire histories,
// remains the assertion; synchronization must not normalize the observation.
func TestL07PropertyDisconnectConfig(t *testing.T) {
	l07CheckWireRows(t, "r_property_disconnect_config")
}

// Keep full native histories for the delivery boundaries, including repeated
// cached values, suppression, errors, and LDS intermediate snapshots.
func TestL07WireDeliverySynchronization(t *testing.T) {
	l07CheckWireRows(t,
		"r_property_null", "r_property_undefined", "r_property_ordinal_nan",
		"r_property_number", "r_property_object", "r_property_ordinal_fraction", "r_property_null_return",
		"r_property_config_same", "r_function_config_mutate",
		"r_property_rapid", "r_function_rapid_suppress", "r_property_cache_return",
		"r_property_error_then_value", "r_function_error_then_value", "r_property_value_then_error",
		"r_function_disconnect_config",
		"r_lds_property_notify_unchanged", "r_lds_function_notify_changed",
		"r_lds_property_reconnect_notify", "r_lds_function_disconnected_notify",
		"r_lds_property_pending_to_cached", "r_lds_function_pending_to_undefined",
		"r_lds_property_error_to_cached", "r_lds_function_error_to_undefined",
		"r_lds_property_switch_back")
}

func l07CheckWireRows(t *testing.T, ids ...string) {
	t.Helper()
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	data, err := os.ReadFile("testdata/l07_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l07Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	var selected []l07Case
	for _, row := range cases {
		if wanted[row.ID] {
			selected = append(selected, row)
		}
	}
	if len(selected) != len(wanted) {
		t.Fatalf("expected %d wire rows, got %d", len(wanted), len(selected))
	}
	root, err := gladehome.Root()
	if err != nil {
		t.Fatal(err)
	}
	l07CheckRuntimeBuild(t)
	versions := []string{"59.0", "67.0", "68.0"}
	observed, err := l07ObserveDOM(t, versions, selected, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range versions {
		t.Run(api, func(t *testing.T) {
			result := observed[api]
			if result.CompileErr != nil || len(result.Errors) != 0 || len(result.Values) != len(selected) {
				t.Fatalf("wire observation unavailable: compile=%v errors=%v values=%d", result.CompileErr, result.Errors, len(result.Values))
			}
			for _, row := range selected {
				t.Run(row.ID, func(t *testing.T) {
					got, err := l07BrowserText(result.Values[row.ID])
					if err != nil {
						t.Fatal(err)
					}
					if want := row.Expected[api]; got != want {
						t.Fatalf("wire expected <%s> actual <%s>", want, got)
					}
				})
			}
		})
	}
}
