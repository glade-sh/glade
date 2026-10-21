public class ChoiceBoundaryProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> observed = new Map<String,Object>();
        observed.put('mapped_without_controller', updateChoice(null, 'Allowed'));
        observed.put('mapped_with_controller', updateChoice('Red', 'Allowed'));
        observed.put('mapped_with_other_controller', updateChoice('Blue', 'Allowed'));
        observed.put('unmapped_without_controller', updateChoice(null, 'Loose'));
        Map<Id, Schema.RecordTypeInfo> byId = TypeBoundary__c.SObjectType.getDescribe().getRecordTypeInfosById();
        List<String> keyNames = new List<String>();
        List<Boolean> keyMasterFlags = new List<Boolean>();
        for (Id key : byId.keySet()) {
            keyNames.add(byId.get(key).getDeveloperName());
            keyMasterFlags.add(byId.get(key).isMaster());
        }
        List<String> listNames = new List<String>();
        for (Schema.RecordTypeInfo info : TypeBoundary__c.SObjectType.getDescribe().getRecordTypeInfos()) {
            listNames.add(info.getDeveloperName());
        }
        observed.put('record_type_key_names', keyNames);
        observed.put('record_type_key_master_flags', keyMasterFlags);
        observed.put('record_type_list_names', listNames);
        return observed;
    }
    private static Map<String,Object> updateChoice(String controller, String choice) {
        Map<String,Object> result = new Map<String,Object>();
        ChoiceBoundary__c row = new ChoiceBoundary__c(Name='Owned choice', Control__c=controller);
        Database.SaveResult inserted = Database.insert(row, false, AccessLevel.SYSTEM_MODE);
        result.put('insert_success', inserted.isSuccess());
        if (!inserted.isSuccess()) {
            List<String> errors = new List<String>();
            List<String> messages = new List<String>();
            for (Database.Error error : inserted.getErrors()) {
                errors.add(String.valueOf(error.getStatusCode()));
                messages.add(error.getMessage());
            }
            result.put('insert_errors', errors);
            result.put('insert_messages', messages);
            return result;
        }
        row.Choice__c = choice;
        try {
            Database.update(row, AccessLevel.SYSTEM_MODE);
            result.put('update_success', true);
            ChoiceBoundary__c stored = [SELECT Control__c, Choice__c FROM ChoiceBoundary__c WHERE Id=:row.Id WITH SYSTEM_MODE];
            result.put('stored_controller', stored.Control__c);
            result.put('stored_choice', stored.Choice__c);
        } catch (DmlException error) {
            result.put('update_success', false);
            result.put('type', error.getTypeName());
            result.put('status', String.valueOf(error.getDmlStatusCode(0)));
            result.put('message', error.getDmlMessage(0));
        }
        return result;
    }
}
