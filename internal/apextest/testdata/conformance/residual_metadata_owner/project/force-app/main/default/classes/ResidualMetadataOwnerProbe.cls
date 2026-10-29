public class ResidualMetadataOwnerProbe {
 public static Map<String,Object> observe(){
  gladens001__Ns113bFlag__mdt row=[SELECT DeveloperName,NamespacePrefix,QualifiedApiName FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='BoundaryOwner20261026'];
  gladens001__Ns113bFlag__mdt bare=gladens001__Ns113bFlag__mdt.getInstance('BoundaryOwner20261026');
  gladens001__Ns113bFlag__mdt owner=gladens001__Ns113bFlag__mdt.getInstance('gladens002__BoundaryOwner20261026');
  return new Map<String,Object>{'owner'=>row.NamespacePrefix,'qualified'=>row.QualifiedApiName,'bare'=>bare==null?null:bare.QualifiedApiName,'qualifiedOwner'=>owner==null?null:owner.QualifiedApiName,'providerMissing'=>gladens001__Ns113bFlag__mdt.getInstance('gladens001__BoundaryOwner20261026')==null,'missing'=>gladens001__Ns113bFlag__mdt.getInstance('AbsentBoundary20261026')==null,'allContains'=>gladens001__Ns113bFlag__mdt.getAll().containsKey('BoundaryOwner20261026')};
 }
}
