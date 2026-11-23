try {
  RegulatoryAuthorizationType t = new RegulatoryAuthorizationType(Name='t2', RegulatoryAuthCategory='License'); insert t;
  BusinessLicenseApplication b = new BusinessLicenseApplication(LicenseTypeId=t.Id, AppliedDate=DateTime.now()); insert b;
  System.debug('R5|bla_nort|Category=' + [SELECT Category FROM BusinessLicenseApplication WHERE Id=:b.Id].Category);
} catch (Exception e) { System.debug('R5|bla_nort|EXC|' + e.getMessage()); }
