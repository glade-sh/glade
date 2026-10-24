public class MixedDmlPartialShapes {
    private static String status(Boolean success,Database.Error[] errors) {
        return success ? 'success' : String.valueOf(errors[0].getStatusCode());
    }
    public static Map<String,Object> observe(String route) {
        String customerName='Shape '+route;
        List<Account> records;
        if (route=='undelete_scalar') {
            records=[SELECT Id,Name FROM Account WHERE Name=:customerName AND IsDeleted=true ORDER BY Id ALL ROWS];
        } else {
            records=[SELECT Id,Name FROM Account WHERE Name=:customerName ORDER BY Id];
        }
        String suffix=route.replace('_','')+String.valueOf(Datetime.now().getTime());
        insert new PermissionSet(Name='Shape'+suffix,Label='Shape '+suffix);
        List<String> statuses=new List<String>();
        String surface='result';
        try {
            if (route=='delete_scalar') { Database.DeleteResult r=Database.delete(records[0],false); statuses.add(status(r.isSuccess(),r.getErrors())); }
            else if (route=='insert_scalar') { Database.SaveResult r=Database.insert(new Account(Name='Shape rejected'),false); statuses.add(status(r.isSuccess(),r.getErrors())); }
            else if (route=='update_scalar') { Database.SaveResult r=Database.update(records[0],false); statuses.add(status(r.isSuccess(),r.getErrors())); }
            else if (route=='upsert_insert_scalar') { Database.UpsertResult r=Database.upsert(new Account(Name='Shape rejected'),false); statuses.add(status(r.isSuccess(),r.getErrors())); }
            else if (route=='upsert_update_scalar') { Database.UpsertResult r=Database.upsert(records[0],false); statuses.add(status(r.isSuccess(),r.getErrors())); }
            else if (route=='undelete_scalar') { Database.UndeleteResult r=Database.undelete(records[0],false); statuses.add(status(r.isSuccess(),r.getErrors())); }
            else if (route=='merge_list') { Database.MergeResult[] results=Database.merge(records[0],new List<Account>{records[1]},false); for (Database.MergeResult r:results) { statuses.add(status(r.isSuccess(),r.getErrors())); } }
            else if (route=='insert_mixed_setup_first') { Database.SaveResult[] results=Database.insert(new List<SObject>{new PermissionSet(Name='Mixed'+suffix,Label='Mixed '+suffix),new Account(Name='Shape rejected')},false); for (Database.SaveResult r:results) { statuses.add(status(r.isSuccess(),r.getErrors())); } }
            else if (route=='insert_mixed_data_first') { Database.SaveResult[] results=Database.insert(new List<SObject>{new Account(Name='Shape rejected'),new PermissionSet(Name='Mixed'+suffix,Label='Mixed '+suffix)},false); for (Database.SaveResult r:results) { statuses.add(status(r.isSuccess(),r.getErrors())); } }
        } catch (DmlException e) {
            surface='exception'; statuses.add(e.getTypeName());
        }
        insert new PermissionSet(Name='AfterShape'+suffix,Label='After shape '+suffix);
        return new Map<String,Object>{'surface'=>surface,'statuses'=>statuses,'remaining'=>[SELECT COUNT() FROM Account WHERE Name=:customerName]};
    }
}
