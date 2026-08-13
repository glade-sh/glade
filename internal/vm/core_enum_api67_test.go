package vm

import "testing"

func TestExecAPI67CoreEnumValuesAndOrdinals(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> loggingLevelNames = new List<String>{'NONE', 'INTERNAL', 'FINEST', 'FINER', 'FINE', 'DEBUG', 'INFO', 'WARN', 'ERROR'};
List<LoggingLevel> loggingLevels = LoggingLevel.values();
System.assertEquals(loggingLevelNames.size(), loggingLevels.size());
for (Integer i = 0; i < loggingLevels.size(); i++) {
    System.assertEquals(loggingLevelNames.get(i), loggingLevels.get(i).name());
    System.assertEquals(i, loggingLevels.get(i).ordinal());
    LoggingLevel resolved = LoggingLevel.valueOf(loggingLevelNames.get(i).toLowerCase());
	System.assert(loggingLevels.get(i).equals(resolved), 'LoggingLevel equals ' + String.valueOf(i));
    System.assertEquals(loggingLevels.get(i).hashCode(), resolved.hashCode());
}

System.assert(!loggingLevels.get(0).equals(loggingLevels.get(1)));

List<String> triggerOperationNames = new List<String>{'BEFORE_INSERT', 'AFTER_INSERT', 'BEFORE_UPDATE', 'AFTER_UPDATE', 'BEFORE_DELETE', 'AFTER_DELETE', 'AFTER_UNDELETE'};
List<TriggerOperation> triggerOperations = TriggerOperation.values();
System.assertEquals(triggerOperationNames.size(), triggerOperations.size());
for (Integer i = 0; i < triggerOperations.size(); i++) {
    System.assertEquals(triggerOperationNames.get(i), triggerOperations.get(i).name());
    System.assertEquals(i, triggerOperations.get(i).ordinal());
    TriggerOperation resolved = TriggerOperation.valueOf(triggerOperationNames.get(i).toLowerCase());
	System.assert(triggerOperations.get(i).equals(resolved), 'TriggerOperation equals ' + String.valueOf(i));
    System.assertEquals(triggerOperations.get(i).hashCode(), resolved.hashCode());
}

System.assert(!triggerOperations.get(0).equals(triggerOperations.get(1)));

List<String> jsonTokenNames = new List<String>{'NOT_AVAILABLE', 'START_OBJECT', 'END_OBJECT', 'START_ARRAY', 'END_ARRAY', 'FIELD_NAME', 'VALUE_EMBEDDED_OBJECT', 'VALUE_STRING', 'VALUE_NUMBER_INT', 'VALUE_NUMBER_FLOAT', 'VALUE_TRUE', 'VALUE_FALSE', 'VALUE_NULL'};
List<JSONToken> jsonTokens = JSONToken.values();
System.assertEquals(jsonTokenNames.size(), jsonTokens.size());
for (Integer i = 0; i < jsonTokens.size(); i++) {
    System.assertEquals(jsonTokenNames.get(i), jsonTokens.get(i).name());
    System.assertEquals(i, jsonTokens.get(i).ordinal());
    JSONToken resolved = JSONToken.valueOf(jsonTokenNames.get(i).toLowerCase());
	System.assert(jsonTokens.get(i).equals(resolved), 'JSONToken equals ' + String.valueOf(i));
    System.assertEquals(jsonTokens.get(i).hashCode(), resolved.hashCode());
}

System.assert(!jsonTokens.get(0).equals(jsonTokens.get(1)));

List<String> roundingModeNames = new List<String>{'UP', 'DOWN', 'CEILING', 'FLOOR', 'HALF_UP', 'HALF_DOWN', 'HALF_EVEN', 'UNNECESSARY'};
List<RoundingMode> roundingModes = RoundingMode.values();
System.assertEquals(roundingModeNames.size(), roundingModes.size());
for (Integer i = 0; i < roundingModes.size(); i++) {
    System.assertEquals(roundingModeNames.get(i), roundingModes.get(i).name());
    System.assertEquals(i, roundingModes.get(i).ordinal());
    RoundingMode resolved = RoundingMode.valueOf(roundingModeNames.get(i).toLowerCase());
	System.assert(roundingModes.get(i).equals(resolved), 'RoundingMode equals ' + String.valueOf(i));
    System.assertEquals(roundingModes.get(i).hashCode(), resolved.hashCode());
}
System.assert(!roundingModes.get(0).equals(roundingModes.get(1)));

List<String> accessTypeNames = new List<String>{'CREATABLE', 'READABLE', 'UPDATABLE', 'UPSERTABLE'};
List<AccessType> accessTypes = AccessType.values();
System.assertEquals(accessTypeNames.size(), accessTypes.size());
for (Integer i = 0; i < accessTypes.size(); i++) {
    System.assertEquals(accessTypeNames.get(i), accessTypes.get(i).name());
    System.assertEquals(i, accessTypes.get(i).ordinal());
    AccessType resolved = AccessType.valueOf(accessTypeNames.get(i).toLowerCase());
	System.assert(accessTypes.get(i).equals(resolved), 'AccessType equals ' + String.valueOf(i));
    System.assertEquals(accessTypes.get(i).hashCode(), resolved.hashCode());
}
System.assert(!accessTypes.get(0).equals(accessTypes.get(1)));

List<String> parentJobResultNames = new List<String>{'SUCCESS', 'UNHANDLED_EXCEPTION'};
List<ParentJobResult> parentJobResults = ParentJobResult.values();
System.assertEquals(parentJobResultNames.size(), parentJobResults.size());
for (Integer i = 0; i < parentJobResults.size(); i++) {
    System.assertEquals(parentJobResultNames.get(i), parentJobResults.get(i).name());
    System.assertEquals(i, parentJobResults.get(i).ordinal());
    ParentJobResult resolved = ParentJobResult.valueOf(parentJobResultNames.get(i).toLowerCase());
	System.assert(parentJobResults.get(i).equals(resolved), 'ParentJobResult equals ' + String.valueOf(i));
    System.assertEquals(parentJobResults.get(i).hashCode(), resolved.hashCode());
}
System.assert(!parentJobResults.get(0).equals(parentJobResults.get(1)));

List<String> quiddityNames = new List<String>{'BATCH_ACS', 'BATCH_CHUNK_SERIAL', 'BATCH_CHUNK_PARALLEL', 'FUTURE', 'SCHEDULED', 'SYNCHRONOUS', 'RUN_INTEGRATION_TESTS', 'RUNTEST_SYNC', 'RUNTEST_ASYNC', 'RUNTEST_DEPLOY', 'VF', 'QUEUEABLE', 'REMOTE_ACTION', 'AURA', 'QUICK_ACTION', 'BULK_API', 'SOAP', 'REST', 'INVOCABLE_ACTION', 'ANONYMOUS', 'INBOUND_EMAIL_SERVICE', 'BATCH_APEX', 'DISCOVERABLE_LOGIN', 'IOT', 'COMMERCE_INTEGRATION', 'TRANSACTION_FINALIZER_QUEUEABLE', 'FUNCTION_CALLBACK', 'POST_INSTALL_SCRIPT', 'PLATFORM_EVENT_PUBLISH_CALLBACK', 'EXTERNAL_SERVICE_CALLBACK', 'TRANSACTION_SECURITY_POLICY', 'UNDEFINED'};
List<Quiddity> quiddities = Quiddity.values();
System.assertEquals(quiddityNames.size(), quiddities.size());
for (Integer i = 0; i < quiddities.size(); i++) {
    System.assertEquals(quiddityNames.get(i), quiddities.get(i).name());
    System.assertEquals(i, quiddities.get(i).ordinal());
    Quiddity resolved = Quiddity.valueOf(quiddityNames.get(i).toLowerCase());
	System.assert(quiddities.get(i).equals(resolved), 'Quiddity equals ' + String.valueOf(i));
    System.assertEquals(quiddities.get(i).hashCode(), resolved.hashCode());
}
System.assert(!quiddities.get(0).equals(quiddities.get(1)));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
