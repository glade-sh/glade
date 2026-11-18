List<Account> rows = [SELECT Id, RecordTypeId, RecordType.DeveloperName FROM Account LIMIT 1];
System.debug('ROW|S3 static SOQL|ok');
