@IsTest private class Road174Review67 {
 @IsTest static void check() {
  Account acc = new Account(Name='Access probe'); insert acc;
  Lead lead = new Lead(LastName='Access probe',Company='Example'); insert lead;
  Campaign campaign = new Campaign(Name='Access probe'); insert campaign;
  Product2 product = new Product2(Name='Access probe',IsActive=true); insert product;
  Asset asset = new Asset(Name='Access probe',AccountId=acc.Id,Product2Id=product.Id); insert asset;
  Opportunity opp = new Opportunity(Name='Access probe',StageName='Prospecting',CloseDate=Date.today(),AccountId=acc.Id,CampaignId=campaign.Id); insert opp;
  List<Lead> r0 = [SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Lead LIMIT 1];
  if (!r0.isEmpty()) { Boolean a=r0[0].UserRecordAccess.HasReadAccess; String b=r0[0].UserRecordAccess.MaxAccessLevel; System.assertNotEquals(null,a); System.assertNotEquals(null,b); }
  List<Campaign> r1 = [SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Campaign LIMIT 1];
  if (!r1.isEmpty()) { Boolean a=r1[0].UserRecordAccess.HasReadAccess; String b=r1[0].UserRecordAccess.MaxAccessLevel; System.assertNotEquals(null,a); System.assertNotEquals(null,b); }
  List<Contract> r2 = [SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Contract LIMIT 1];
  if (!r2.isEmpty()) { Boolean a=r2[0].UserRecordAccess.HasReadAccess; String b=r2[0].UserRecordAccess.MaxAccessLevel; System.assertNotEquals(null,a); System.assertNotEquals(null,b); }
  List<Asset> r3 = [SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Asset LIMIT 1];
  if (!r3.isEmpty()) { Boolean a=r3[0].UserRecordAccess.HasReadAccess; String b=r3[0].UserRecordAccess.MaxAccessLevel; System.assertNotEquals(null,a); System.assertNotEquals(null,b); }
  List<Product2> r4 = [SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Product2 LIMIT 1];
  if (!r4.isEmpty()) { Boolean a=r4[0].UserRecordAccess.HasReadAccess; String b=r4[0].UserRecordAccess.MaxAccessLevel; System.assertNotEquals(null,a); System.assertNotEquals(null,b); }
  List<Pricebook2> r5 = [SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Pricebook2 LIMIT 1];
  if (!r5.isEmpty()) { Boolean a=r5[0].UserRecordAccess.HasReadAccess; String b=r5[0].UserRecordAccess.MaxAccessLevel; System.assertNotEquals(null,a); System.assertNotEquals(null,b); }
  List<Order> r6 = [SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Order LIMIT 1];
  if (!r6.isEmpty()) { Boolean a=r6[0].UserRecordAccess.HasReadAccess; String b=r6[0].UserRecordAccess.MaxAccessLevel; System.assertNotEquals(null,a); System.assertNotEquals(null,b); }
  List<OpportunityLineItem> r7 = [SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM OpportunityLineItem LIMIT 1];
  if (!r7.isEmpty()) { Boolean a=r7[0].UserRecordAccess.HasReadAccess; String b=r7[0].UserRecordAccess.MaxAccessLevel; System.assertNotEquals(null,a); System.assertNotEquals(null,b); }
  List<Opportunity> nested = [SELECT Campaign.UserRecordAccess.HasReadAccess,Campaign.UserRecordAccess.HasEditAccess FROM Opportunity WHERE Id=:opp.Id];
  System.assertNotEquals(null,nested[0].Campaign.UserRecordAccess.HasReadAccess);
  List<Asset> nestedAsset = [SELECT Product2.UserRecordAccess.HasReadAccess,Product2.UserRecordAccess.HasEditAccess FROM Asset WHERE Id=:asset.Id];
  System.assertNotEquals(null,nestedAsset[0].Product2.UserRecordAccess.HasReadAccess);
  Boolean rejected=false; try { Database.query('SELECT UserRecordAccess.UserId FROM Lead'); } catch (QueryException e) { rejected=true; } System.assert(rejected);
 }
}
