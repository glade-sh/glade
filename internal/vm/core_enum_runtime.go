package vm

import "strings"

var api67CoreEnumOrders = map[string][]string{
	"AccessType": {
		"CREATABLE", "READABLE", "UPDATABLE", "UPSERTABLE",
	},
	"JSONToken": {
		"NOT_AVAILABLE", "START_OBJECT", "END_OBJECT", "START_ARRAY", "END_ARRAY",
		"FIELD_NAME", "VALUE_EMBEDDED_OBJECT", "VALUE_STRING", "VALUE_NUMBER_INT",
		"VALUE_NUMBER_FLOAT", "VALUE_TRUE", "VALUE_FALSE", "VALUE_NULL",
	},
	"LoggingLevel": {
		"NONE", "INTERNAL", "FINEST", "FINER", "FINE", "DEBUG", "INFO", "WARN", "ERROR",
	},
	"ParentJobResult": {
		"SUCCESS", "UNHANDLED_EXCEPTION",
	},
	"Quiddity": {
		"BATCH_ACS", "BATCH_CHUNK_SERIAL", "BATCH_CHUNK_PARALLEL", "FUTURE", "SCHEDULED",
		"SYNCHRONOUS", "RUN_INTEGRATION_TESTS", "RUNTEST_SYNC", "RUNTEST_ASYNC", "RUNTEST_DEPLOY",
		"VF", "QUEUEABLE", "REMOTE_ACTION", "AURA", "QUICK_ACTION", "BULK_API", "SOAP", "REST",
		"INVOCABLE_ACTION", "ANONYMOUS", "INBOUND_EMAIL_SERVICE", "BATCH_APEX", "DISCOVERABLE_LOGIN",
		"IOT", "COMMERCE_INTEGRATION", "TRANSACTION_FINALIZER_QUEUEABLE", "FUNCTION_CALLBACK",
		"POST_INSTALL_SCRIPT", "PLATFORM_EVENT_PUBLISH_CALLBACK", "EXTERNAL_SERVICE_CALLBACK",
		"TRANSACTION_SECURITY_POLICY", "UNDEFINED",
	},
	"RoundingMode": {
		"UP", "DOWN", "CEILING", "FLOOR", "HALF_UP", "HALF_DOWN", "HALF_EVEN", "UNNECESSARY",
	},
	"TriggerOperation": {
		"BEFORE_INSERT", "AFTER_INSERT", "BEFORE_UPDATE", "AFTER_UPDATE", "BEFORE_DELETE",
		"AFTER_DELETE", "AFTER_UNDELETE",
	},
}

func coreEnumSpec(typeName string) (string, []string, bool) {
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	for canonical, names := range api67CoreEnumOrders {
		if strings.EqualFold(typeName, canonical) {
			return canonical, names, true
		}
	}
	return "", nil, false
}

var loggingLevelNames = api67CoreEnumOrders["LoggingLevel"]
var triggerOperationNames = api67CoreEnumOrders["TriggerOperation"]
var jsonTokenNames = api67CoreEnumOrders["JSONToken"]
var roundingModeNames = api67CoreEnumOrders["RoundingMode"]
var accessTypeNames = api67CoreEnumOrders["AccessType"]
