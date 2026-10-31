package server

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestVisualforceOnDemandRuntimeSharesRegistration(t *testing.T) {
	apextest.InvalidateRuntimeCaches()
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	org := storage.NewOrgState()
	host := New(&org)
	index := typesys.Index{}
	host.SetProjectRuntime(index, nil, nil)
	first, err := host.visualforceRuntime()
	if err != nil {
		t.Fatal(err)
	}
	second, err := host.visualforceRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Methods) == 0 || reflect.ValueOf(first.Methods).Pointer() != reflect.ValueOf(second.Methods).Pointer() {
		t.Fatal("on-demand Visualforce requests rebuilt standard registration")
	}
	if err := first.RegisterMethod(vm.Method{Name: "PrivateRow.method"}); err != nil {
		t.Fatal(err)
	}
	if _, leaked := second.Methods["PrivateRow.method"]; leaked {
		t.Fatal("request registration leaked into another request")
	}
	third, err := host.visualforceRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := third.Methods["PrivateRow.method"]; leaked {
		t.Fatal("request registration leaked into the template")
	}
}
