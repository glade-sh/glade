public class CloneFlagsObservation {
    public class Bundle { public List<SObject> rows; }
    private static String fields(SObject record) {
        List<String> names = new List<String>(record.getPopulatedFieldsAsMap().keySet());
        names.sort();
        return String.join(names, ',');
    }
    private static String save(SObject record, Boolean updateRecord) {
        Database.SaveResult result;
        if (updateRecord) result = Database.update(record, false);
        else result = Database.insert(record, false);
        if (result.isSuccess()) return 'success';
        Database.Error error = result.getErrors()[0];
        return String.valueOf(error.getStatusCode()) + ':' + String.join(error.getFields(), ',');
    }
    private static String saveUser(SObject record, Boolean updateRecord) {
        Database.SaveResult result;
        if (updateRecord) result = Database.update(record, false, AccessLevel.USER_MODE);
        else result = Database.insert(record, false, AccessLevel.USER_MODE);
        if (result.isSuccess()) return 'success';
        Database.Error error = result.getErrors()[0];
        return String.valueOf(error.getStatusCode()) + ':' + String.join(error.getFields(), ',');
    }
    public static List<String> observe() {
        List<String> rows = new List<String>();
        CloneSample__c seed = new CloneSample__c(Name='Clone source');
        insert seed;
        CloneSample__c source = [SELECT Id, Name, IsDeleted, CreatedById, CreatedDate,
            LastModifiedById, LastModifiedDate, SystemModstamp, Sequence__c
            FROM CloneSample__c WHERE Id=:seed.Id];
        rows.add('source\t' + fields(source));
        rows.add('single_default\t' + fields(source.clone()));
        rows.add('list_default\t' + fields(new List<CloneSample__c>{source}.deepClone()[0]));
        for (Boolean keepId : new List<Boolean>{false,true}) {
            for (Boolean keepDates : new List<Boolean>{false,true}) {
                for (Boolean keepAuto : new List<Boolean>{false,true}) {
                    String flags = String.valueOf(keepId) + '_' + String.valueOf(keepDates) + '_' + String.valueOf(keepAuto);
                    for (Boolean deep : new List<Boolean>{false,true}) {
                        rows.add('single_' + flags + '_' + String.valueOf(deep) + '\t' + fields(source.clone(keepId,deep,keepDates,keepAuto)));
                    }
                    rows.add('list_' + flags + '\t' + fields(new List<CloneSample__c>{source}.deepClone(keepId,keepDates,keepAuto)[0]));
                }
            }
        }
        rows.add('insert_default\t' + save(source.clone(),false));
        rows.add('insert_list\t' + save(new List<CloneSample__c>{source}.deepClone()[0],false));
        Bundle bundle = new Bundle(); bundle.rows = new List<SObject>{source};
        Bundle restored = (Bundle)JSON.deserialize(JSON.serialize(bundle), Bundle.class);
        rows.add('insert_wrapper_double_clone\t' + save(restored.rows[0].clone().clone(),false));
        CloneSample__c noId = source.clone(false,false,true,true);
        String payload = JSON.serialize(noId);
        rows.add('insert_preserved\t' + save(noId,false));
        rows.add('insert_json\t' + save((CloneSample__c)JSON.deserialize(payload,CloneSample__c.class),false));
        rows.add('update_preserved\t' + save(source.clone(true,false,true,true),true));
        rows.add('update_json\t' + save((CloneSample__c)JSON.deserialize(JSON.serialize(source),CloneSample__c.class),true));
        for (String field : new List<String>{'CreatedById','CreatedDate','LastModifiedById','LastModifiedDate','SystemModstamp','IsDeleted','Sequence__c'}) {
            Map<String,Object> values = new Map<String,Object>{'Name'=>'Field control',field=>source.get(field)};
            rows.add('insert_json_' + field + '\t' + save((CloneSample__c)JSON.deserialize(JSON.serialize(values),CloneSample__c.class),false));
            values.put('Id',source.Id);
            rows.add('update_json_' + field + '\t' + save((CloneSample__c)JSON.deserialize(JSON.serialize(values),CloneSample__c.class),true));
        }
        for (String field : new List<String>{'CreatedDate','LastModifiedDate','SystemModstamp','IsDeleted'}) {
            Object supplied = field == 'IsDeleted' ? (Object)true : (Object)Datetime.newInstanceGmt(2001,2,3,4,5,6);
            Map<String,Object> values = new Map<String,Object>{'Name'=>'Historical control',field=>supplied};
            CloneSample__c inserted = (CloneSample__c)JSON.deserialize(JSON.serialize(values),CloneSample__c.class);
            rows.add('insert_historical_' + field + '\t' + save(inserted,false));
            CloneSample__c stored = Database.query('SELECT ' + field + ' FROM CloneSample__c WHERE Id=\'' + inserted.Id + '\'');
            rows.add('insert_historical_stored_' + field + '\t' + String.valueOf(stored.get(field) == supplied));
            values.put('Id',source.Id);
            CloneSample__c updated = (CloneSample__c)JSON.deserialize(JSON.serialize(values),CloneSample__c.class);
            rows.add('update_historical_' + field + '\t' + save(updated,true));
            stored = Database.query('SELECT ' + field + ' FROM CloneSample__c WHERE Id=\'' + source.Id + '\'');
            rows.add('update_historical_stored_' + field + '\t' + String.valueOf(stored.get(field) == supplied));
        }
        for (Boolean reverse : new List<Boolean>{false,true}) {
            String pair = reverse ? '"FormulaZ__c":2,"FormulaA__c":1' : '"FormulaA__c":1,"FormulaZ__c":2';
            CloneSample__c invalid = (CloneSample__c)JSON.deserialize('{"Name":"Formula control",' + pair + '}',CloneSample__c.class);
            rows.add('insert_formula_' + String.valueOf(reverse) + '\t' + save(invalid,false));
            invalid.Id = source.Id;
            rows.add('update_formula_' + String.valueOf(reverse) + '\t' + save(invalid,true));
        }
        rows.add('insert_user_default\t' + saveUser(source.clone(),false));
        rows.add('insert_user_list\t' + saveUser(new List<CloneSample__c>{source}.deepClone()[0],false));
        rows.add('insert_user_preserved\t' + saveUser(source.clone(false,false,true,true),false));
        rows.add('update_user_preserved\t' + saveUser(source.clone(true,false,true,true),true));
        rows.add('source_after\t' + fields(source));
        return rows;
    }
}
