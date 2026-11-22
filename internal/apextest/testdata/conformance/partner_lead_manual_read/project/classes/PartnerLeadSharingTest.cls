@IsTest private class PartnerLeadSharingTest {
 static String observe(User u, Id a, Id b) {
  List<UserRecordAccess> rows=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:u.Id AND RecordId=:a];
  String bits=rows.isEmpty()?'none':String.valueOf(rows[0].HasReadAccess)+','+String.valueOf(rows[0].HasEditAccess)+','+String.valueOf(rows[0].HasDeleteAccess)+','+String.valueOf(rows[0].HasTransferAccess)+','+String.valueOf(rows[0].HasAllAccess)+','+rows[0].MaxAccessLevel;
  Integer countRows; Integer queryRows;
  System.runAs(u) { countRows=PartnerLeadReader.countIds(new Set<Id>{a,b}); queryRows=PartnerLeadReader.queryIds(new Set<Id>{a,b}); }
  return bits+':'+countRows+','+queryRows;
 }
 @IsTest static void observe() {
  String result='unset';
  try {
   PclSupport.setup();
   Contact c=[SELECT AccountId FROM Contact WHERE LastName='PclCon' LIMIT 1];
   Account a=[SELECT Id,IsPartner FROM Account WHERE Id=:c.AccountId];
   System.runAs(PclSupport.owner) { a.IsPartner=true; update a; }
   User u=PclSupport.portal('Partner Community User'); PclLeadGrant.grant(u);
   Lead first=new Lead(LastName='First',Company='Synthetic'); Lead second=new Lead(LastName='Second',Company='Synthetic');
   System.runAs(PclSupport.owner) { insert new List<Lead>{first,second}; }
   String before=observe(u,first.Id,second.Id);
   System.runAs(PclSupport.owner) { insert new LeadShare(LeadId=first.Id,UserOrGroupId=u.Id,LeadAccessLevel='Read',RowCause='Manual'); }
   String after=observe(u,first.Id,second.Id);
   result='before='+before+'|after='+after;
  } catch(Exception e) { result='exception='+e.getTypeName()+':'+e.getMessage(); }
  System.assert(false,result);
 }
}
