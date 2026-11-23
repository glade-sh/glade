try {
  RegulatoryAuthorizationType t = new RegulatoryAuthorizationType(Name='t', RegulatoryAuthCategory='License'); insert t;
  Id rt = Schema.SObjectType.BusinessLicenseApplication.getRecordTypeInfosByDeveloperName().get('RT').getRecordTypeId();
  BusinessLicenseApplication b = new BusinessLicenseApplication(RecordTypeId=rt, LicenseTypeId=t.Id, AppliedDate=DateTime.now()); insert b;
  System.debug('R5|bla_rt|Category=' + [SELECT Category FROM BusinessLicenseApplication WHERE Id=:b.Id].Category);
} catch (Exception e) { System.debug('R5|bla_rt|EXC|' + e.getMessage()); }
