public class Road174Context62 {
 public static String collect(String mode) {
  Map<String,Object> out=new Map<String,Object>();
  for(String own:new List<String>{'own','other'}) {
   Id uid=UserInfo.getUserId();String cond=own=='own'?' = :uid':' != :uid'; String suffix=mode=='default'?'':' WITH '+mode+'_MODE';
   try {List<SObject> q=Database.query('SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Order WHERE OwnerId'+cond+suffix);List<Object> v=new List<Object>();if(!q.isEmpty()){SObject a=q[0].getSObject('UserRecordAccess');for(String f:new List<String>{'HasReadAccess','HasEditAccess','HasDeleteAccess','HasTransferAccess','HasAllAccess','MaxAccessLevel'})v.add(a.get(f));}out.put(own,v);}catch(Exception e){out.put(own,'ERROR:'+e.getTypeName()+':'+e.getMessage());}
  }return JSON.serialize(out);
 }
}