public class MixedDmlActions {
    public static Map<String,Object> observe(String route) {
        String customerName='Action '+route;
        List<Account> records;
        if (route=='undelete_database_partial') {
            records=[SELECT Id,Name,Description FROM Account WHERE Name=:customerName AND IsDeleted=true ORDER BY Id ALL ROWS];
        } else {
            records=[SELECT Id,Name,Description FROM Account WHERE Name=:customerName ORDER BY Id];
        }
        String suffix=route.replace('_','')+String.valueOf(Datetime.now().getTime());
        insert new PermissionSet(Name='Action'+suffix,Label='Action '+suffix);
        String status='success';
        String surface='completed';
        try {
            if (route=='delete_statement_scalar') { delete records[0]; }
            else if (route=='delete_statement_list') { delete records; }
            else if (route=='delete_database_scalar') { Database.delete(records[0]); }
            else if (route=='delete_database_list') { Database.delete(records); }
            else if (route=='delete_database_id') { Database.delete(records[0].Id); }
            else if (route=='delete_database_ids') { Database.delete(new List<Id>{records[0].Id,records[1].Id}); }
            else if (route=='delete_database_all_or_none') { Database.delete(records,true); }
            else if (route=='delete_database_partial') { Database.DeleteResult[] results=Database.delete(records,false); surface='result'; status=results[0].isSuccess() ? 'success' : String.valueOf(results[0].getErrors()[0].getStatusCode()); }
            else if (route=='merge_statement_scalar') { merge records[0] records[1]; }
            else if (route=='merge_statement_list') { merge records[0] new List<Account>{records[1]}; }
            else if (route=='merge_database_scalar') { Database.merge(records[0],records[1]); }
            else if (route=='merge_database_list') { Database.merge(records[0],new List<Account>{records[1]}); }
            else if (route=='merge_database_all_or_none') { Database.merge(records[0],records[1],true); }
            else if (route=='merge_database_partial') { Database.MergeResult result=Database.merge(records[0],records[1],false); surface='result'; status=result.isSuccess() ? 'success' : String.valueOf(result.getErrors()[0].getStatusCode()); }
            else if (route=='insert_rejection') { insert new Account(Name='Action rejected'); }
            else if (route=='update_rejection') { records[0].Description='Changed'; update records[0]; }
            else if (route=='insert_database_partial') { Database.SaveResult[] results=Database.insert(new List<Account>{new Account(Name='Action rejected')},false); surface='result'; status=results[0].isSuccess() ? 'success' : String.valueOf(results[0].getErrors()[0].getStatusCode()); }
            else if (route=='update_database_partial') { Database.SaveResult[] results=Database.update(records,false); surface='result'; status=results[0].isSuccess() ? 'success' : String.valueOf(results[0].getErrors()[0].getStatusCode()); }
            else if (route=='upsert_database_insert_partial') { Database.UpsertResult[] results=Database.upsert(new List<Account>{new Account(Name='Action rejected')},false); surface='result'; status=results[0].isSuccess() ? 'success' : String.valueOf(results[0].getErrors()[0].getStatusCode()); }
            else if (route=='upsert_database_update_partial') { Database.UpsertResult[] results=Database.upsert(records,false); surface='result'; status=results[0].isSuccess() ? 'success' : String.valueOf(results[0].getErrors()[0].getStatusCode()); }
            else if (route=='undelete_database_partial') { Database.UndeleteResult[] results=Database.undelete(records,false); surface='result'; status=results[0].isSuccess() ? 'success' : String.valueOf(results[0].getErrors()[0].getStatusCode()); }
        } catch (DmlException e) {
            surface='exception';
            status=(e.getMessage().contains('MIXED_DML_OPERATION') || e.getMessage().contains('Mixed DML'))
                ? 'MIXED_DML_OPERATION' : e.getTypeName()+': '+e.getMessage();
        }
        insert new PermissionSet(Name='After'+suffix,Label='After '+suffix);
        return new Map<String,Object>{'status'=>status,'surface'=>surface,'remaining'=>[SELECT COUNT() FROM Account WHERE Name=:customerName]};
    }
}
