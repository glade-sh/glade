@IsTest private class Road174OrderBoundary62 {

 private static List<Object> vector(SObject s) { List<Object> out=new List<Object>(); for(String f:new List<String>{'HasReadAccess','HasEditAccess','HasDeleteAccess','HasTransferAccess','HasAllAccess','MaxAccessLevel'}) out.add(s.get(f)); return out; }
 @IsTest static void check() {
  Account a=new Account(Name='Order boundary');insert a;
  Order o=new Order(AccountId=a.Id,EffectiveDate=Date.today(),Status='Draft');insert o;
  Id rid=o.Id;Id uid=UserInfo.getUserId();Map<String,Object> out=new Map<String,Object>();
  List<Order> q=[SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Order WHERE Id=:rid];
  out.put('plain_relationship',vector(q[0].UserRecordAccess));
  List<UserRecordAccess> u=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId=:rid];out.put('plain_direct',vector(u[0]));
  System.runAs(new User(Id=uid)) {
   List<Order> r=[SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Order WHERE Id=:rid];out.put('runas_relationship',vector(r[0].UserRecordAccess));
   List<UserRecordAccess> d=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId=:rid];out.put('runas_direct',vector(d[0]));
  }
  System.assert(false,'ROAD174O|'+JSON.serialize(out));
 }
}
