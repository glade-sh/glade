String unitName = 'N1A' + String.valueOf(DateTime.now().getTime());
FlowCoreN1Record__c subject = new FlowCoreN1Record__c(
    Name = 'subject', Unit__c = unitName, Text__c = 'seed', Flag__c = false, Trace__c = 'seed'
);
insert subject;
subject = [SELECT Text__c, Flag__c, Trace__c FROM FlowCoreN1Record__c WHERE Id = :subject.Id];
Map<String, Object> payload = new Map<String, Object>{
    'text' => subject.Text__c,
    'textIsNull' => subject.Text__c == null,
    'flag' => subject.Flag__c,
    'trace' => subject.Trace__c
};
System.assert(false, 'FLOW_CORE_RESULT|N1_001|' + JSON.serialize(payload));
