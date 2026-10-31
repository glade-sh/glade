@IsTest private class NoRowDynamicControlTest {
 @IsTest static void isolated() {
  insert new Pricebook2(Name='Owned Dynamic 2711',IsActive=true);
  Map<String,Object> binds=new Map<String,Object>{'name'=>'Owned Dynamic 2711'};
  String std='SELECT Id FROM Pricebook2 WHERE IsStandard = true';
  String created='SELECT Id FROM Pricebook2 WHERE Name = :name';
  String countStd='SELECT COUNT() FROM Pricebook2 WHERE IsStandard = true';
  String countCreated='SELECT COUNT() FROM Pricebook2 WHERE Name = :name';
  Map<String,Object> result=new Map<String,Object>();
  result.put('plain',Database.query(std).size());
  result.put('system',Database.query(std,AccessLevel.SYSTEM_MODE).size());
  result.put('user',Database.query(std,AccessLevel.USER_MODE).size());
  result.put('bindSystem',Database.queryWithBinds(std,binds,AccessLevel.SYSTEM_MODE).size());
  result.put('bindUser',Database.queryWithBinds(std,binds,AccessLevel.USER_MODE).size());
  result.put('createdSystem',Database.queryWithBinds(created,binds,AccessLevel.SYSTEM_MODE).size());
  result.put('createdUser',Database.queryWithBinds(created,binds,AccessLevel.USER_MODE).size());
  result.put('count',Database.countQuery(countStd));
  result.put('countSystem',Database.countQuery(countStd,AccessLevel.SYSTEM_MODE));
  result.put('countUser',Database.countQuery(countStd,AccessLevel.USER_MODE));
  result.put('countBindSystem',Database.countQueryWithBinds(countStd,binds,AccessLevel.SYSTEM_MODE));
  result.put('countBindUser',Database.countQueryWithBinds(countStd,binds,AccessLevel.USER_MODE));
  result.put('countCreatedSystem',Database.countQueryWithBinds(countCreated,binds,AccessLevel.SYSTEM_MODE));
  result.put('countCreatedUser',Database.countQueryWithBinds(countCreated,binds,AccessLevel.USER_MODE));
  String missing='none';
  try { Pricebook2 p=Database.query('SELECT Id FROM Pricebook2 WHERE Name = \'Owned missing 2711\''); }
  catch (QueryException e) { missing=e.getMessage(); }
  result.put('missing',missing);
  System.assert(false,'OBSERVED|isolated|'+JSON.serialize(result));
 }
 @IsTest(SeeAllData=true) static void allData() {
  String q='SELECT Id FROM Pricebook2 WHERE IsStandard = true';
  Map<String,Object> empty=new Map<String,Object>();
  System.assert(false,'OBSERVED|allData|'+JSON.serialize(new Map<String,Object>{'plain'=>Database.query(q).size(),'system'=>Database.query(q,AccessLevel.SYSTEM_MODE).size(),'user'=>Database.query(q,AccessLevel.USER_MODE).size(),'bindSystem'=>Database.queryWithBinds(q,empty,AccessLevel.SYSTEM_MODE).size(),'bindUser'=>Database.queryWithBinds(q,empty,AccessLevel.USER_MODE).size()}));
 }
}
