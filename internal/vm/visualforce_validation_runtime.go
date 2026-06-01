package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/dml"
)

func (vm *VM) addVisualforceDMLPageMessages(results []dml.Result) {
	if vm == nil || vm.currentPage.Kind == "" {
		return
	}
	for _, result := range results {
		if result.Success || !strings.EqualFold(result.StatusCode, "REQUIRED_FIELD_MISSING") || strings.TrimSpace(result.Error) == "" {
			continue
		}
		message := Object("ApexPages.Message")
		severity, _ := apexPagesSeverityStaticValue("ApexPages.Severity.ERROR")
		message.Fields["severity"] = severity
		message.Fields["summary"] = String(result.Error)
		message.Fields["detail"] = String(result.Error)
		vm.addApexPageMessage(message)
	}
}
