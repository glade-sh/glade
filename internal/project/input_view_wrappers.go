package project

import (
	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/inputview"
)

// PathInsideRoot reads straight from disk. PathInsideRootWithInputs observes its reads through v.
func PathInsideRoot(root, full string) bool {
	return pathInsideRoot(nil, root, full)
}

func PathInsideRootWithInputs(v *inputview.View, root, full string) bool {
	return pathInsideRoot(v, root, full)
}

// ScratchCountryDefaults reads straight from disk. ScratchCountryDefaultsWithInputs observes its reads through v.
func ScratchCountryDefaults(root string) (string, config.TestUserConfig, bool) {
	return scratchCountryDefaults(nil, root)
}

func ScratchCountryDefaultsWithInputs(v *inputview.View, root string) (string, config.TestUserConfig, bool) {
	return scratchCountryDefaults(v, root)
}

// ScratchCountryUserDefaults reads straight from disk. ScratchCountryUserDefaultsWithInputs observes its reads through v.
func ScratchCountryUserDefaults(root string, user config.TestUserConfig) config.TestUserConfig {
	return scratchCountryUserDefaults(nil, root, user)
}

func ScratchCountryUserDefaultsWithInputs(v *inputview.View, root string, user config.TestUserConfig) config.TestUserConfig {
	return scratchCountryUserDefaults(v, root, user)
}

// ScratchObjectDefaultRecordTypes reads straight from disk. ScratchObjectDefaultRecordTypesWithInputs observes its reads through v.
func ScratchObjectDefaultRecordTypes(root string) map[string]string {
	return scratchObjectDefaultRecordTypes(nil, root)
}

func ScratchObjectDefaultRecordTypesWithInputs(v *inputview.View, root string) map[string]string {
	return scratchObjectDefaultRecordTypes(v, root)
}

// OrgDomainURL reads straight from disk. OrgDomainURLWithInputs observes its reads through v.
func OrgDomainURL(root string) (string, error) {
	return orgDomainURL(nil, root)
}

func OrgDomainURLWithInputs(v *inputview.View, root string) (string, error) {
	return orgDomainURL(v, root)
}

// OrgRuntimeSettings reads straight from disk. OrgRuntimeSettingsWithInputs observes its reads through v.
func OrgRuntimeSettings(root string) (string, []string, error) {
	return orgRuntimeSettings(nil, root)
}

func OrgRuntimeSettingsWithInputs(v *inputview.View, root string) (string, []string, error) {
	return orgRuntimeSettings(v, root)
}

// OrgShapeFeatures reads straight from disk. OrgShapeFeaturesWithInputs observes its reads through v.
func OrgShapeFeatures(root string) []string {
	return orgShapeFeatures(nil, root)
}

func OrgShapeFeaturesWithInputs(v *inputview.View, root string) []string {
	return orgShapeFeatures(v, root)
}
