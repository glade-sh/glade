if ([SELECT COUNT() FROM DpA__c] == 0) {
    insert new DpA__c(Name = 'A1');
}
if ([SELECT COUNT() FROM DpB__c] == 0) {
    insert new DpB__c(Name = 'B1');
}
System.debug('OBS|seed|a=' + [SELECT COUNT() FROM DpA__c] + '|b=' + [SELECT COUNT() FROM DpB__c]);
