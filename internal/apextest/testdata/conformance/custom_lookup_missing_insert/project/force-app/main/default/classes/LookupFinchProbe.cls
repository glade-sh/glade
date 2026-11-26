@IsTest
private class LookupFinchProbe {
    @IsTest static void observe() {
        Savepoint beforeProbe = Database.setSavepoint();
        LookupBirch__c birch = new LookupBirch__c(Name='Live birch');
        insert birch;
        Account account = new Account(Name='Live account');
        insert account;
        Id missingBirch = (Id)(LookupBirch__c.SObjectType.getDescribe().getKeyPrefix() + '000000000zzz');
        Id missingAccount = (Id)('001000000000zzz');
        List<LookupFinch__c> birds = new List<LookupFinch__c>{
            new LookupFinch__c(Name='missing_custom', Birch__c=missingBirch),
            new LookupFinch__c(Name='missing_standard', Perch__c=missingAccount),
            new LookupFinch__c(Name='existing_custom', Birch__c=birch.Id),
            new LookupFinch__c(Name='existing_standard', Perch__c=account.Id),
            new LookupFinch__c(Name='clear_custom', Birch__c=null)
        };
        List<Database.SaveResult> results = Database.insert(birds, false);
        Map<String, String> rows = new Map<String, String>();
        for (Integer i = 0; i < birds.size(); i++) {
            Database.SaveResult row = results[i];
            String value = String.valueOf(row.isSuccess()) + '|' + (row.getId() != null)
                + '|' + (birds[i].Id != null) + '|' + row.getErrors().size();
            if (!row.isSuccess()) {
                Database.Error error = row.getErrors()[0];
                String message = error.getMessage();
                Id target = i == 0 ? missingBirch : missingAccount;
                message = message.replace(String.valueOf(target), '<target18>')
                    .replace(String.valueOf(target).substring(0,15), '<target15>');
                value += '|' + error.getStatusCode() + '|' + String.join(error.getFields(), ',') + '|' + message;
            }
            rows.put(birds[i].Name, value);
        }
        LookupFinch__c statementBird = new LookupFinch__c(Name='statement_custom', Birch__c=missingBirch);
        try {
            insert statementBird;
            rows.put('statement_custom', 'success|' + (statementBird.Id != null));
        } catch (DmlException error) {
            String message = error.getDmlMessage(0).replace(String.valueOf(missingBirch), '<target18>')
                .replace(String.valueOf(missingBirch).substring(0,15), '<target15>');
            rows.put('statement_custom', 'DmlException|' + (statementBird.Id != null)
                + '|' + error.getNumDml() + '|' + error.getDmlIndex(0) + '|' + error.getDmlType(0)
                + '|' + String.join(error.getDmlFieldNames(0), ',') + '|' + message);
        }
        Database.rollback(beforeProbe);
        System.assert(false, 'OBS|' + JSON.serialize(rows));
    }
}
