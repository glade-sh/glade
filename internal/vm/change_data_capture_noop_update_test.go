package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Only a merge drops the update event that changes no field; an ordinary update
// that writes the stored value back still delivers its event.
func TestChangeDataCaptureNoChangeUpdateKeepsEvent(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = api + ".0"
			storage.EnsureStandardObject(&org, "Contact")
			machine := changeDataCaptureCounterMachine(t, &org, "ContactChangeEvent")
			machine.EnableTestContext()
			source := fmt.Sprintf(`Test.enableChangeDataCapture(); Contact c = new Contact(LastName='same'); insert c; Test.getEventBus().deliver(); CaptureCounter.hits = 0; c.LastName = 'same'; update c; Test.getEventBus().deliver(); System.assertEquals(%d, CaptureCounter.hits);`, 1)
			executeChangeDataCaptureSource(t, machine, api, source)
		})
	}
}
