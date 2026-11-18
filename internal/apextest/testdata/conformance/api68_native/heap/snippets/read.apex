List<Capture_Row__c> rows = [SELECT Stage__c, Body__c FROM Capture_Row__c WHERE Case__c = 'heap' ORDER BY CreatedDate];
for (Capture_Row__c r : rows) {
    System.debug(LoggingLevel.ERROR, 'ROW|' + r.Stage__c + '|' + r.Body__c);
}
System.debug(LoggingLevel.ERROR, 'ROW_COUNT=' + rows.size());
