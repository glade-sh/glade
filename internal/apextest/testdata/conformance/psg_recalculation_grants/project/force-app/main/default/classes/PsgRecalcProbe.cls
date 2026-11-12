@IsTest private class PsgRecalcProbe {
 @IsTest static void observe() {
  Map<String,Object> result = new Map<String,Object>();
  try {
   PermissionSet grant = new PermissionSet(Name='RecalcGrant', Label='Recalc Grant', PermissionsApiEnabled=true); insert grant;
   PermissionSetGroup g = new PermissionSetGroup(DeveloperName='RecalcProbe', MasterLabel='Recalc Probe'); insert g;
   insert new PermissionSetGroupComponent(PermissionSetGroupId=g.Id,PermissionSetId=grant.Id);
   result.put('afterComponent', [SELECT Status FROM PermissionSetGroup WHERE Id=:g.Id].Status);
   try { Test.calculatePermissionSetGroup(g.Id); result.put('calculate','success'); } catch (Exception e) { result.put('calculate', e.getTypeName()+': '+e.getMessage()); }
   result.put('afterCalculate', [SELECT Status FROM PermissionSetGroup WHERE Id=:g.Id].Status); result.put('grantAfterCalculate',[SELECT PermissionsApiEnabled FROM PermissionSet WHERE PermissionSetGroupId=:g.Id AND Type='Group' LIMIT 1].PermissionsApiEnabled);
   delete [SELECT Id FROM PermissionSetGroupComponent WHERE PermissionSetGroupId=:g.Id AND PermissionSetId=:grant.Id];
   result.put('afterRemoval', [SELECT Status FROM PermissionSetGroup WHERE Id=:g.Id].Status);
   try { Test.calculatePermissionSetGroup(g.Id); result.put('recalculate','success'); } catch (Exception e) { result.put('recalculate', e.getTypeName()+': '+e.getMessage()); }
   result.put('afterRecalculate', [SELECT Status FROM PermissionSetGroup WHERE Id=:g.Id].Status); result.put('grantAfterRecalculate',[SELECT PermissionsApiEnabled FROM PermissionSet WHERE PermissionSetGroupId=:g.Id AND Type='Group' LIMIT 1].PermissionsApiEnabled);
  } catch (Exception e) { result.put('setupError',e.getTypeName()+': '+e.getMessage()); }
  System.assert(false,'CAP|'+JSON.serialize(result));
 }
}
