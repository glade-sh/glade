Savepoint point=Database.setSavepoint();AsyncParcel__c row=[SELECT Id,Name FROM AsyncParcel__c LIMIT 1];row.Name='update';update row;Database.rollback(point);
