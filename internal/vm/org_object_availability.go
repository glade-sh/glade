package vm

import (
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Native unlicensed CampaignInfluence has a separate test-context describe
// surface when its setting is enabled. Other objects use the org policy.
func (vm *VM) allowsStandardObject(name string) bool {
	if vm == nil || vm.Org == nil {
		return true
	}
	if vm.Org.HasFeature(project.PlainDeveloperObjectShape) && (!storage.PlainDeveloperObjectPresent(name, vm.plainDeveloperObjectMask()) || vm.plainDeveloperObjectBeforeCapturedAPI(name)) {
		return false
	}
	if strings.EqualFold(name, "CampaignInfluence") && vm.testContext != nil &&
		vm.Org.HasFeature("CampaignInfluence2Enabled") {
		return true
	}
	if (vm.Org.HasFeature("OrderManagement") || vm.Org.HasCapturedB2BCommerceShape()) && !vm.Org.HasFeature(project.UnspecifiedStandardFieldShape) {
		switch {
		case strings.EqualFold(name, "WebCartCredit"):
			if apexversion.Before(vm.currentMethod.APIVersion, 64) {
				return false
			}
		case strings.EqualFold(name, "PaymentCredit"), strings.EqualFold(name, "PaymentCreditLinePayment"), strings.EqualFold(name, "PaymentCreditTransaction"):
			if apexversion.Before(vm.currentMethod.APIVersion, 65) {
				return false
			}
		}
	}
	return vm.Org.AllowsStandardObject(name)
}

// Global describe is shared only between caller versions with the same
// captured commerce-object availability. Unspecified shapes retain fallback.
func (vm *VM) globalDescribeAvailabilityKey() string {
	if vm != nil && vm.Org != nil && vm.Org.HasFeature(project.PlainDeveloperObjectShape) {
		key := "plain" + strconv.Itoa(int(vm.plainDeveloperObjectMask()))
		if major, ok := apexversion.Major(vm.currentMethod.APIVersion); ok && major > 62 && major < 67 {
			key += "@" + strconv.Itoa(major)
		}
		return key
	}
	if vm == nil || vm.Org == nil || (!vm.Org.HasFeature("OrderManagement") && !vm.Org.HasCapturedB2BCommerceShape()) || vm.Org.HasFeature(project.UnspecifiedStandardFieldShape) {
		return ""
	}
	if apexversion.Before(vm.currentMethod.APIVersion, 64) {
		return "before64"
	}
	if apexversion.Before(vm.currentMethod.APIVersion, 65) {
		return "api64"
	}
	return "api65plus"
}

// plainDeveloperObjectMask selects the captured plain Developer inventory for
// the caller's API version and route (plain_global_describe): the 62.0 list
// through API 62, the 67.0 list from API 67, and either between them, since no
// version between was captured. Test context adds the isTest-only objects.
func (vm *VM) plainDeveloperObjectMask() uint8 {
	api := vm.currentMethod.APIVersion
	var anonymous, testOnly uint8
	switch {
	case apexversion.Before(api, 63):
		anonymous, testOnly = storage.PlainDeveloperAnonymous62, storage.PlainDeveloperTestOnly62
	case !apexversion.Before(api, 67):
		anonymous, testOnly = storage.PlainDeveloperAnonymous67, storage.PlainDeveloperTestOnly67
	default:
		anonymous = storage.PlainDeveloperAnonymous62 | storage.PlainDeveloperAnonymous67
		testOnly = storage.PlainDeveloperTestOnly62 | storage.PlainDeveloperTestOnly67
	}
	if vm.testContext != nil {
		return anonymous | testOnly
	}
	return anonymous
}

// plainDeveloperObjectMinimumAPI: the first API whose plain Developer global
// describe and Task/Event WhatId targets list the object
// (whatid_targets_tiebreak global8 rows, orgs A and B, anonymous 62.0 to 68.0).
// The 62.0 and 67.0 lists already decide the endpoints; this splits the
// union used between them for the captured objects only.
var plainDeveloperObjectMinimumAPI = map[string]int{
	"devopsenvironment":      64,
	"devopsactivitylog":      65,
	"devopsenvdeployment":    65,
	"devopsenvdeploymentmbr": 65,
	"devopsrequestinfo":      65,
	"sfdcpartnersbscroffer":  65,
	"paygroup":               66,
	"payperiod":              66,
}

func (vm *VM) plainDeveloperObjectBeforeCapturedAPI(name string) bool {
	minimum, captured := plainDeveloperObjectMinimumAPI[strings.ToLower(strings.TrimSpace(name))]
	return captured && apexversion.Before(vm.currentMethod.APIVersion, minimum)
}
