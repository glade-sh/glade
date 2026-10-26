public class ResultLiteralProbe {
 public static Map<String,Object> observe() {
 Map<String,Object> result=new Map<String,Object>();
 result.put('Database.SaveResult', Database.SaveResult.class.getName());
 Database.SaveResult value0=(Database.SaveResult)JSON.deserialize('{"success":true}',Database.SaveResult.class);
 result.put('Database.SaveResult.success',value0.isSuccess());
 result.put('Database.UpsertResult', Database.UpsertResult.class.getName());
 Database.UpsertResult value1=(Database.UpsertResult)JSON.deserialize('{"success":true}',Database.UpsertResult.class);
 result.put('Database.UpsertResult.success',value1.isSuccess());
 result.put('Database.DeleteResult', Database.DeleteResult.class.getName());
 Database.DeleteResult value2=(Database.DeleteResult)JSON.deserialize('{"success":true}',Database.DeleteResult.class);
 result.put('Database.DeleteResult.success',value2.isSuccess());
 result.put('Database.UndeleteResult', Database.UndeleteResult.class.getName());
 Database.UndeleteResult value3=(Database.UndeleteResult)JSON.deserialize('{"success":true}',Database.UndeleteResult.class);
 result.put('Database.UndeleteResult.success',value3.isSuccess());
 result.put('Database.EmptyRecycleBinResult', Database.EmptyRecycleBinResult.class.getName());
 Database.EmptyRecycleBinResult value4=(Database.EmptyRecycleBinResult)JSON.deserialize('{"success":true}',Database.EmptyRecycleBinResult.class);
 result.put('Database.EmptyRecycleBinResult.success',value4.isSuccess());
 result.put('Database.LeadConvertResult', Database.LeadConvertResult.class.getName());
 Database.LeadConvertResult value5=(Database.LeadConvertResult)JSON.deserialize('{"success":true}',Database.LeadConvertResult.class);
 result.put('Database.LeadConvertResult.success',value5.isSuccess());
 result.put('Database.MergeResult', Database.MergeResult.class.getName());
 Database.MergeResult value6=(Database.MergeResult)JSON.deserialize('{"success":true}',Database.MergeResult.class);
 result.put('Database.MergeResult.success',value6.isSuccess());
 result.put('Approval.LockResult', Approval.LockResult.class.getName());
 Approval.LockResult value7=(Approval.LockResult)JSON.deserialize('{"success":true}',Approval.LockResult.class);
 result.put('Approval.LockResult.success',value7.isSuccess());
 result.put('Approval.UnlockResult', Approval.UnlockResult.class.getName());
 Approval.UnlockResult value8=(Approval.UnlockResult)JSON.deserialize('{"success":true}',Approval.UnlockResult.class);
 result.put('Approval.UnlockResult.success',value8.isSuccess());
 result.put('Approval.ProcessResult', Approval.ProcessResult.class.getName());
 Approval.ProcessResult value9=(Approval.ProcessResult)JSON.deserialize('{"success":true}',Approval.ProcessResult.class);
 result.put('Approval.ProcessResult.success',value9.isSuccess());
 return result;
 }
}
