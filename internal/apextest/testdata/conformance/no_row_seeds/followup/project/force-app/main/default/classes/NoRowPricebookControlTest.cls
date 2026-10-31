@IsTest private class NoRowPricebookControlTest {
 @IsTest static void isolated() {
  Integer beforeCount = [SELECT COUNT() FROM Pricebook2 WHERE IsStandard = true];
  Id standardId = Test.getStandardPricebookId();
  Integer afterCount = [SELECT COUNT() FROM Pricebook2 WHERE IsStandard = true];
  Integer byId = [SELECT COUNT() FROM Pricebook2 WHERE Id = :standardId];
  insert new Pricebook2(Name = 'Synthetic 2711', IsActive = true);
  Integer inserted = [SELECT COUNT() FROM Pricebook2 WHERE Name = 'Synthetic 2711'];
  String missing = 'none';
  try { Pricebook2 p = [SELECT Id FROM Pricebook2 WHERE Name = 'Synthetic missing 2711' LIMIT 1]; }
  catch (QueryException e) { missing = e.getMessage(); }
  Integer accountCount = [SELECT COUNT() FROM Account WHERE Name = 'Synthetic existing 2711'];
  System.assert(false, 'OBSERVED|isolated|' + JSON.serialize(new Map<String,Object>{'before'=>beforeCount,'after'=>afterCount,'byId'=>byId,'inserted'=>inserted,'missing'=>missing,'accountCount'=>accountCount}));
 }
 @IsTest(SeeAllData=true) static void allData() {
  Integer standardCount = [SELECT COUNT() FROM Pricebook2 WHERE IsStandard = true];
  Integer accountCount = [SELECT COUNT() FROM Account WHERE Name = 'Synthetic existing 2711'];
  System.assert(false, 'OBSERVED|allData|' + JSON.serialize(new Map<String,Object>{'standardCount'=>standardCount,'accountCount'=>accountCount}));
 }
}
