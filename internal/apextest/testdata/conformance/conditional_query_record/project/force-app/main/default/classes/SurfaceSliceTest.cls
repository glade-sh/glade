@IsTest private class SurfaceSliceTest {
 @IsTest static void conditionalQuery() {
  Account seed=new Account(Name='Grove');insert seed;
  Boolean locked=true;
  Account first=locked ? [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1 FOR UPDATE] : [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1];
  locked=false;
  Account second=locked ? [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1 FOR UPDATE] : [SELECT Id FROM Account WHERE Id=:seed.Id LIMIT 1];
  System.assert(false,'OBSERVED|conditionalQuery|'+JSON.serialize(new Map<String,Object>{'first'=>first.Id==seed.Id,'second'=>second.Id==seed.Id}));
 }
}
