package vm

import (
	"fmt"
	"time"

	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/storage"
)

// ConfigureTestEnvironment seeds a private run before test context snapshots,
// static initialization and @TestSetup. It never changes an org template.
func (vm *VM) ConfigureTestEnvironment(settings config.TestConfig) error {
	if err := config.ValidateTestEnvironment(settings); err != nil {
		return err
	}
	if zone := settings.User.TimeZone; zone != "" {
		if _, _, ok := parseFixedTimeZoneID(zone); !ok {
			if _, ok := supportedNamedTimeZone(zone); !ok {
				return fmt.Errorf("test.user.timeZone: %q is not supported by the local datetime runtime", zone)
			}
		}
	}
	if start := settings.Clock.Start; start != "" {
		parsed, _ := time.Parse(time.RFC3339Nano, start)
		vm.testClockStart = parsed.UTC().Truncate(time.Millisecond)
		vm.hasTestClockStart = true
		vm.fakeNow = vm.testClockStart
		vm.lastNow = time.Time{}
		vm.hasLastNow = false
		vm.hasPendingNowMillis = false
	}
	if settings.User == (config.TestUserConfig{}) {
		return nil
	}
	record, ok := vm.defaultOrgUserRecord()
	if !ok {
		return fmt.Errorf("test.user: no default running user in the test org")
	}
	record = record.Clone()
	for field, value := range map[string]string{
		"TimeZoneSidKey": settings.User.TimeZone, "LocaleSidKey": settings.User.Locale,
		"LanguageLocaleKey": settings.User.Language, "DefaultCurrencyIsoCode": settings.User.DefaultCurrencyISOCode,
	} {
		if value != "" {
			record.Fields[field] = storage.StringValue(value)
		}
	}
	users := vm.Org.Objects["User"]
	users.Records[record.ID] = record
	vm.Org.Objects["User"] = users
	return nil
}

func (vm *VM) currentLocalNow() time.Time {
	zone := vm.currentUserTimeZoneID()
	if _, _, local, _, ok := resolveTimeZoneForInstant(zone, vm.fakeNow); ok {
		return local
	}
	if location, err := time.LoadLocation(zone); err == nil {
		return vm.fakeNow.In(location)
	}
	return vm.fakeNow.UTC()
}
