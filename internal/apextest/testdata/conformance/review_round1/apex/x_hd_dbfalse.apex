HdA__c sa = new HdA__c(Name='a'); insert sa; insert new HdB__c(Name='b');
HdCtl.reset('plain');
HdA__c a = [SELECT Id FROM HdA__c LIMIT 1]; a.Name = 'qdb';
Database.SaveResult r = Database.update(a, false);
System.debug('OBS|hd_dbfalse|' + r.isSuccess() + '|' + (r.isSuccess() ? '' : String.valueOf(r.getErrors()[0].getStatusCode())) + '|d=' + HdCtl.depth);