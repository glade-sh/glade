public class ResultTokenProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> rows = new Map<String,Object>();
  rows.put('SaveResultToken', Database.SaveResult.class.getName());
  Database.SaveResult valueSaveResult = (Database.SaveResult)JSON.deserialize('{"success":true,"errors":[]}', Database.SaveResult.class);
  rows.put('SaveResultSuccess', valueSaveResult.isSuccess());
  List<Database.SaveResult> listSaveResult = (List<Database.SaveResult>)JSON.deserialize('[{"success":true,"errors":[]}]', List<Database.SaveResult>.class);
  rows.put('SaveResultListSuccess', listSaveResult[0].isSuccess());
  rows.put('UpsertResultToken', Database.UpsertResult.class.getName());
  Database.UpsertResult valueUpsertResult = (Database.UpsertResult)JSON.deserialize('{"success":true,"errors":[]}', Database.UpsertResult.class);
  rows.put('UpsertResultSuccess', valueUpsertResult.isSuccess());
  List<Database.UpsertResult> listUpsertResult = (List<Database.UpsertResult>)JSON.deserialize('[{"success":true,"errors":[]}]', List<Database.UpsertResult>.class);
  rows.put('UpsertResultListSuccess', listUpsertResult[0].isSuccess());
  rows.put('DeleteResultToken', Database.DeleteResult.class.getName());
  Database.DeleteResult valueDeleteResult = (Database.DeleteResult)JSON.deserialize('{"success":true,"errors":[]}', Database.DeleteResult.class);
  rows.put('DeleteResultSuccess', valueDeleteResult.isSuccess());
  List<Database.DeleteResult> listDeleteResult = (List<Database.DeleteResult>)JSON.deserialize('[{"success":true,"errors":[]}]', List<Database.DeleteResult>.class);
  rows.put('DeleteResultListSuccess', listDeleteResult[0].isSuccess());
  rows.put('LeadConvertResultToken', Database.LeadConvertResult.class.getName());
  Database.LeadConvertResult valueLeadConvertResult = (Database.LeadConvertResult)JSON.deserialize('{"success":true,"errors":[]}', Database.LeadConvertResult.class);
  rows.put('LeadConvertResultSuccess', valueLeadConvertResult.isSuccess());
  List<Database.LeadConvertResult> listLeadConvertResult = (List<Database.LeadConvertResult>)JSON.deserialize('[{"success":true,"errors":[]}]', List<Database.LeadConvertResult>.class);
  rows.put('LeadConvertResultListSuccess', listLeadConvertResult[0].isSuccess());
  Account record = new Account(Name='Result Token Control');
  Database.SaveResult saved = System.Database.insert(record, false);
  rows.put('insertSuccess', saved.isSuccess());
  Database.UpsertResult upserted = System.Database.upsert(record, false);
  rows.put('upsertSuccess', upserted.isSuccess());
  Database.DeleteResult removed = System.Database.delete(record, false);
  rows.put('deleteSuccess', removed.isSuccess());
  return rows;
 }
}
