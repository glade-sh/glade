public class GroupProjectionObservation {
 public static Map<String,Object> observe(Boolean calculate) {
  String nonce=EncodingUtil.convertToHex(Crypto.generateAesKey(128)).substring(0,10);
  String name='OwnedGroup_'+nonce;
  PermissionSetGroup groupRow=new PermissionSetGroup(DeveloperName=name,MasterLabel='Distinct group label '+nonce);
  Map<String,Object> rows=new Map<String,Object>();
  rows.put('beforeInsert',new Map<String,Object>{'matchingCount'=>[SELECT COUNT() FROM PermissionSet WHERE Name=:name]});
  insert groupRow;
  rows.put('afterInsert',snapshot(groupRow));
  PermissionSet standalone=new PermissionSet(Name='OwnedStandalone_'+nonce,Label='Distinct standalone label '+nonce);
  insert standalone;
  PermissionSet standaloneQuery=[SELECT Name,Label,Type,PermissionSetGroupId,IsOwnedByProfile,IsCustom,NamespacePrefix,LicenseId FROM PermissionSet WHERE Id=:standalone.Id];
  rows.put('standalone',new Map<String,Object>{'nameMatches'=>standaloneQuery.Name==standalone.Name,'labelMatches'=>standaloneQuery.Label==standalone.Label,'type'=>standaloneQuery.Type,'groupIdNull'=>standaloneQuery.PermissionSetGroupId==null,'ownedByProfile'=>standaloneQuery.IsOwnedByProfile,'custom'=>standaloneQuery.IsCustom,'namespaceNull'=>standaloneQuery.NamespacePrefix==null,'licenseNull'=>standaloneQuery.LicenseId==null});
  insert new PermissionSetGroupComponent(PermissionSetGroupId=groupRow.Id,PermissionSetId=standalone.Id);
  rows.put('afterComponent',snapshot(groupRow));
  if(calculate) {Test.calculatePermissionSetGroup(groupRow.Id);rows.put('afterCalculate',snapshot(groupRow));}
  return rows;
 }
 public static Map<String,Object> snapshot(PermissionSetGroup groupRow) {
  List<PermissionSet> linked=[SELECT Name,Label,Type,PermissionSetGroupId,IsOwnedByProfile,IsCustom,NamespacePrefix,LicenseId FROM PermissionSet WHERE PermissionSetGroupId=:groupRow.Id];
  List<PermissionSet> named=[SELECT Id FROM PermissionSet WHERE Name=:groupRow.DeveloperName AND NamespacePrefix=null];
  Map<String,Object> row=new Map<String,Object>{'linkedCount'=>linked.size(),'namedCount'=>named.size()};
  if(!linked.isEmpty()) {
   PermissionSet record=linked[0];row.put('nameMatchesDeveloperName',record.Name==groupRow.DeveloperName);row.put('labelMatchesMasterLabel',record.Label==groupRow.MasterLabel);row.put('type',record.Type);row.put('linked',record.PermissionSetGroupId==groupRow.Id);row.put('ownedByProfile',record.IsOwnedByProfile);row.put('custom',record.IsCustom);row.put('namespaceNull',record.NamespacePrefix==null);row.put('licenseNull',record.LicenseId==null);
  }
  return row;
 }
}
