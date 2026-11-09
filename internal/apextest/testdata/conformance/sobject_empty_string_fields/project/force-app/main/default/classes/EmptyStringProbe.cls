public class EmptyStringProbe {
    static String shape(Object value) {
        if (value == null) return 'null';
        return '[' + String.valueOf(value) + ']';
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        List<List<String>> specs = new List<List<String>>{
            new List<String>{'Account', 'Name', 'text'}, new List<String>{'Account', 'AccountNumber', 'text'},
            new List<String>{'Account', 'Description', 'longtext'}, new List<String>{'Account', 'BillingStreet', 'textarea'},
            new List<String>{'Account', 'Phone', 'phone'}, new List<String>{'Account', 'Website', 'url'},
            new List<String>{'Account', 'Type', 'picklist'}, new List<String>{'Account', 'Industry', 'picklist'},
            new List<String>{'Contact', 'Email', 'email'}, new List<String>{'Contact', 'Salutation', 'picklist'},
            new List<String>{'Contact', 'Title', 'text'}, new List<String>{'Contact', 'AccountId', 'lookup'},
            new List<String>{'Lead', 'Email', 'email'}, new List<String>{'Lead', 'Status', 'picklist'}
        };
        for (List<String> spec : specs) {
            String objectName = spec[0];
            String fieldName = spec[1];
            Map<String,Object> row = new Map<String,Object>();
            try {
                SObject record = Schema.getGlobalDescribe().get(objectName).newSObject();
                if (objectName == 'Account') record.put('Name', 'EmptyString Probe');
                if (objectName == 'Contact') record.put('LastName', 'EmptyString Probe');
                if (objectName == 'Lead') { record.put('LastName', 'EmptyString Probe'); record.put('Company', 'EmptyString Probe Co'); }
                if (fieldName != 'Name' && fieldName != 'LastName') {
                    try { record.put(fieldName, 'seed'); } catch (Exception ignored) { }
                }
                if (fieldName == 'Name') record.put('Name', 'seed');
                record.put(fieldName, '');
                Object memory = record.get(fieldName);
                row.put('memory', shape(memory));
                row.put('memoryIsNull', memory == null);
            } catch (Exception issue) { row.put('memory', issue.getTypeName() + ':' + issue.getMessage()); }
            result.put(objectName + '.' + fieldName, row);
        }
        // Direct member assignment and the constructor form.
        Contact direct = new Contact(LastName = 'Direct', Email = '');
        result.put('constructorEmail', shape(direct.Email));
        direct.Title = '';
        result.put('directTitle', shape(direct.Title));
        direct.Salutation = '';
        result.put('directSalutation', shape(direct.Salutation));
        direct.Email = 'a@b.example';
        direct.Email = '';
        result.put('reassignedEmail', shape(direct.Email));
        // Persisted values after insert, queried back.
        Account stored = new Account(Name = 'EmptyString Stored', Description = '', Phone = '', Website = '', Type = '', AccountNumber = '', BillingStreet = '', Industry = '');
        insert as system stored;
        Account loaded = [SELECT Description, Phone, Website, Type, AccountNumber, BillingStreet, Industry FROM Account WHERE Id = :stored.Id WITH SYSTEM_MODE];
        result.put('storedDescription', shape(loaded.Description));
        result.put('storedPhone', shape(loaded.Phone));
        result.put('storedWebsite', shape(loaded.Website));
        result.put('storedType', shape(loaded.Type));
        result.put('storedAccountNumber', shape(loaded.AccountNumber));
        result.put('storedBillingStreet', shape(loaded.BillingStreet));
        result.put('storedIndustry', shape(loaded.Industry));
        Contact storedContact = new Contact(LastName = 'EmptyString Stored', Email = '', Title = '', Salutation = '', Description = '', MailingStreet = '');
        insert as system storedContact;
        Contact loadedContact = [SELECT Email, Title, Salutation, Description, MailingStreet FROM Contact WHERE Id = :storedContact.Id WITH SYSTEM_MODE];
        result.put('storedContactEmail', shape(loadedContact.Email));
        result.put('storedContactTitle', shape(loadedContact.Title));
        result.put('storedContactSalutation', shape(loadedContact.Salutation));
        result.put('storedContactDescription', shape(loadedContact.Description));
        result.put('storedContactMailingStreet', shape(loadedContact.MailingStreet));
        storedContact.Title = 'T';
        storedContact.Description = 'D';
        update as system storedContact;
        storedContact.Title = '';
        storedContact.Description = '';
        update as system storedContact;
        Contact reloaded = [SELECT Title, Description FROM Contact WHERE Id = :storedContact.Id WITH SYSTEM_MODE];
        result.put('updatedContactTitle', shape(reloaded.Title));
        result.put('updatedContactDescription', shape(reloaded.Description));
        result.put('afterInsertMemoryDescription', shape(stored.Description));
        result.put('afterInsertMemoryType', shape(stored.Type));
        return result;
    }
}
