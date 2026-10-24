public class CdcGapObservation {
    public static void record(SObject event, EventBus.ChangeEventHeader header) {
        String value = '';
        if (header.changeType != 'DELETE') {
            if (header.entityName == 'Account') value = String.valueOf(event.get('Name'));
            if (header.entityName == 'Contact') value = String.valueOf(event.get('LastName'));
            if (header.entityName == 'CdcItem__c') value = String.valueOf(event.get('Value__c'));
        }
        insert new CdcLog__c(Name='event', Data__c=header.entityName+'|'+header.changeType+'|'+header.recordIds.size()+'|'+value);
    }

    public static Map<String,Object> observe(String key) {
        Map<String,Object> out = new Map<String,Object>{'exception'=>null};
        try {
            if (key == 'setup_a_first' || key == 'setup_a_second' || key == 'setup_b_first') {
                out.put('setupRecords', [SELECT count() FROM Account WHERE Name LIKE 'setup-%']);
                Test.enableChangeDataCapture();
                Test.getEventBus().deliver();
            } else if (key == 'setup_selected') {
                out.put('setupRecords', [SELECT count() FROM Contact WHERE LastName='setup-selected']);
                Test.getEventBus().deliver();
            } else if (key == 'selected_direct') {
                insert new Contact(LastName='selected-direct');
                Test.getEventBus().deliver();
            } else if (key == 'cascade_delete') {
                Account parent = new Account(Name='cascade-parent');
                insert parent;
                insert new Contact(LastName='cascade-child', AccountId=parent.Id);
                Test.enableChangeDataCapture();
                Test.getEventBus().deliver();
                delete [SELECT Id FROM CdcLog__c];
                delete parent;
                Test.getEventBus().deliver();
                out.put('remainingChildren', [SELECT count() FROM Contact WHERE LastName='cascade-child']);
            } else if (key == 'merge_accounts') {
                Account survivor = new Account(Name='merge-survivor');
                Account loser = new Account(Name='merge-loser');
                insert new List<Account>{survivor, loser};
                Test.enableChangeDataCapture();
                Test.getEventBus().deliver();
                delete [SELECT Id FROM CdcLog__c];
                survivor.Name = 'merge-final';
                merge survivor loser;
                Test.getEventBus().deliver();
                out.put('remainingAccounts', [SELECT count() FROM Account WHERE Name LIKE 'merge-%']);
            } else if (key == 'external_upsert') {
                Test.enableChangeDataCapture();
                CdcItem__c item = new CdcItem__c(Name='upsert-item', External__c='owned-key', Value__c='initial');
                upsert item CdcItem__c.External__c;
                item = new CdcItem__c(Name='upsert-item', External__c='owned-key', Value__c='updated');
                upsert item CdcItem__c.External__c;
                Test.getEventBus().deliver();
                out.put('itemCount', [SELECT count() FROM CdcItem__c WHERE External__c='owned-key']);
            }
            List<String> events = new List<String>();
            for (CdcLog__c row : [SELECT Data__c FROM CdcLog__c WHERE Name='event']) events.add(row.Data__c);
            events.sort();
            out.put('events', events);
        } catch (Exception failure) {
            out.put('exception', failure.getTypeName());
        }
        return out;
    }

    public static void emit(String key) {
        System.assert(false, 'CDC_GAP_OBS|'+key+'|'+JSON.serialize(observe(key)));
    }
}
