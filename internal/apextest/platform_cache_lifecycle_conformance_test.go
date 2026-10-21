package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func platformCacheCapturedLifecycle(t *testing.T, runner *conformanceRunner, index typesys.Index, cell platformCacheCapturedCell, api string, reader ir.Program) bool {
	t.Helper()
	switch cell.Flow {
	case "":
		return false
	case "paired":
		if len(cell.PairedMethods) != 2 {
			t.Fatal("incomplete paired test observation")
		}
		var values map[string]struct {
			Outcome string
			Value   json.RawMessage
		}
		if err := json.Unmarshal(cell.Value, &values); err != nil {
			t.Fatal(err)
		}
		// Both methods share the registered class and org seed. Each method
		// uses the same fresh-runtime boundary as the product test runner.
		for _, method := range []string{"first", "second"} {
			observation, exists := values[method]
			if !exists || observation.Outcome != "value" || cell.PairedMethods[method] == "" {
				t.Fatal("missing paired method answer")
			}
			methodCell := cell
			methodCell.ID = cell.PairedMethods[method]
			methodCell.Value = observation.Value
			platformCacheCapturedValue(t, methodCell, platformCacheCapturedNamed(t, runner, index, cell.Source.Tests, method))
		}
	case "async":
		machine := runner.newMachine()
		result, err := machine.Execute(reader)
		if err != nil {
			t.Fatalf("enqueue transaction failed: %v", err)
		}
		if err := machine.DrainAsync(&result); err != nil {
			t.Fatalf("async delivery failed: %v", err)
		}
		if machine.Org == nil || cell.AsyncPrefix == "" {
			t.Fatal("missing async effect context")
		}
		parts := map[int]string{}
		done, hashes := 0, 0
		var digest string
		for _, record := range machine.Org.Objects["Account"].Records {
			name := record.Fields["Name"].String
			if !strings.HasPrefix(name, cell.AsyncPrefix) {
				continue
			}
			tail := strings.TrimPrefix(name, cell.AsyncPrefix)
			switch {
			case tail == "done":
				done++
			case strings.HasPrefix(tail, "hash:"):
				hashes++
				digest = strings.TrimPrefix(tail, "hash:")
			default:
				offset, chunk, valid := strings.Cut(tail, ":")
				start, err := strconv.Atoi(offset)
				if !valid || err != nil || start < 0 || start%180 != 0 {
					t.Fatalf("invalid async chunk: %s", tail)
				}
				if _, duplicate := parts[start]; duplicate {
					t.Fatal("duplicate async chunk")
				}
				parts[start] = chunk
			}
		}
		if done != 1 || hashes != 1 || len(parts) == 0 {
			t.Fatal("missing guarded async effect")
		}
		var payload strings.Builder
		for start := 0; start < len(parts)*180; start += 180 {
			part, exists := parts[start]
			if !exists {
				t.Fatal("async chunk gap")
			}
			payload.WriteString(part)
		}
		sum := sha256.Sum256([]byte(payload.String()))
		if digest != hex.EncodeToString(sum[:]) {
			t.Fatal("async effect digest mismatch")
		}
		platformCacheCapturedPayload(t, cell, payload.String())
	case "requests":
		seed := runner.newOrg()
		runWriter := func(source string, rollback bool) {
			if source == "" {
				return
			}
			if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
				t.Fatalf("writer semantics: %v", analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			transaction := seed.CloneRuntime()
			machine := runner.newMachine()
			machine.SetRuntimeTemplateOrg(&transaction)
			_, err = machine.Execute(program)
			if rollback {
				var thrown *vm.RuntimeError
				if !errors.As(err, &thrown) || platformCacheCapturedQualifiedType(thrown.Type) != cell.WriterExceptionType || thrown.ExceptionMessage() != cell.WriterExceptionMessage {
					t.Fatalf("missing rollback writer exception: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("committed writer failed: %v", err)
			}
			seed = transaction
		}
		runWriter(cell.BeforeAnonymous, false)
		runWriter(cell.RollbackWriter, true)
		machine := runner.newMachine()
		transaction := seed.CloneRuntime()
		machine.SetRuntimeTemplateOrg(&transaction)
		machine.AdvanceDeterministicTime(time.Duration(cell.DelaySeconds) * time.Second)
		_, err := machine.Execute(reader)
		platformCacheCapturedValue(t, cell, err)
	default:
		t.Fatalf("unknown captured lifecycle: %s", cell.Flow)
	}
	return true
}
