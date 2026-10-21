package vm

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// The API65 anonymous/named Task describe captures retain this common order.
// Type was available only in the named capture, after Description; ordering it
// here does not introduce a field absent from the active schema. Catalog-only
// and project fields retain the existing alphabetical order after captured
// fields. Their native availability and ordinals remain unobserved.
var capturedTaskDescribeFieldOrder65 = []string{
	"Id", "WhoId", "WhatId", "Subject", "ActivityDate", "Status", "Priority",
	"IsHighPriority", "OwnerId", "Description", "Type", "IsDeleted", "AccountId",
	"IsClosed", "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById",
	"SystemModstamp", "IsArchived", "CallDurationInSeconds", "CallType",
	"CallDisposition", "CallObject", "ReminderDateTime", "IsReminderSet",
	"RecurrenceActivityId", "IsRecurrence", "RecurrenceStartDateOnly",
	"RecurrenceEndDateOnly", "RecurrenceTimeZoneSidKey", "RecurrenceType",
	"RecurrenceInterval", "RecurrenceDayOfWeekMask", "RecurrenceDayOfMonth",
	"RecurrenceInstance", "RecurrenceMonthOfYear", "RecurrenceRegeneratedType",
	"TaskSubtype", "CompletedDateTime",
}

func (vm *VM) sortCapturedTaskDescribeFieldNames(objectName string, names []string) bool {
	if !vm.usesCapturedTaskDescribeFieldOrder(objectName) {
		return false
	}
	ordinals := make(map[string]int, len(capturedTaskDescribeFieldOrder65))
	for index, name := range capturedTaskDescribeFieldOrder65 {
		ordinals[name] = index
	}
	sort.SliceStable(names, func(i, j int) bool {
		left, leftKnown := ordinals[names[i]]
		right, rightKnown := ordinals[names[j]]
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown {
			return left < right
		}
		return names[i] < names[j]
	})
	return true
}

func (vm *VM) usesCapturedTaskDescribeFieldOrder(objectName string) bool {
	if vm == nil || vm.Org == nil || vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" || !strings.EqualFold(objectName, "Task") {
		return false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	return apexversion.AtLeast(version, 65) && !apexversion.AtLeast(version, 66)
}
