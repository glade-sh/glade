@IsTest private class MdVariantTest {
  @IsTest static void tr() { TrP__c p = new TrP__c(Name='p'); insert p; String r; try { TrC__c x = new TrC__c(Name='n', P__c=p.Id, Amt__c=1); insert x; x.Amt__c=2; update x; r='ok'; } catch (Exception e) { r=e.getTypeName(); } System.assert(false, 'Tr|' + r); }
  @IsTest static void ru() { RuP__c p = new RuP__c(Name='p'); insert p; String r; try { RuC__c x = new RuC__c(Name='n', P__c=p.Id, Amt__c=1); insert x; x.Amt__c=2; update x; r='ok'; } catch (Exception e) { r=e.getTypeName(); } System.assert(false, 'Ru|' + r); }
  @IsTest static void bo() { BoP__c p = new BoP__c(Name='p'); insert p; String r; try { BoC__c x = new BoC__c(Name='n', P__c=p.Id, Amt__c=1); insert x; x.Amt__c=2; update x; r='ok'; } catch (Exception e) { r=e.getTypeName(); } System.assert(false, 'Bo|' + r); }
}
