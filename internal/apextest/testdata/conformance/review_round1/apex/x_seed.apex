if ([SELECT COUNT() FROM HdA__c] == 0) { insert new HdA__c(Name='a'); }
if ([SELECT COUNT() FROM HdB__c] == 0) { insert new HdB__c(Name='b'); }
System.debug('OBS|seed|a=' + [SELECT COUNT() FROM HdA__c] + '|b=' + [SELECT COUNT() FROM HdB__c]);