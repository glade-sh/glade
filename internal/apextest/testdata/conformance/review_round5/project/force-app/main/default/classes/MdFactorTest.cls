@IsTest private class MdFactorTest {
  @IsTest static void fa() { FaP__c p = new FaP__c(Name='p'); insert p; String r; try { FaC__c x = new FaC__c(Name='n', P__c=p.Id, Amt__c=1); insert x; x.Amt__c=2; update x; r='ok'; } catch (Exception e) { r=e.getTypeName(); } System.assert(false, 'Fa|' + r); }
  @IsTest static void fb() { FbP__c p = new FbP__c(Name='p'); insert p; String r; try { FbC__c x = new FbC__c(Name='n', P__c=p.Id, Amt__c=1, Inc__c=true); insert x; x.Amt__c=2; update x; r='ok'; } catch (Exception e) { r=e.getTypeName(); } System.assert(false, 'Fb|' + r); }
  @IsTest static void fc() { FcP__c p = new FcP__c(Name='p'); insert p; String r; try { FcC__c x = new FcC__c(Name='n', P__c=p.Id, Amt__c=1); insert x; RstLog.reset(); x.Amt__c=2; update x; r='ok'; } catch (Exception e) { r=e.getTypeName(); } System.assert(false, 'Fc|' + r); }
  @IsTest static void fd() { FdP__c p = new FdP__c(Name='p'); insert p; String r; try { FdC__c x = new FdC__c(Name='n', P__c=p.Id, Amt__c=1, Inc__c=true); insert x; RstLog.reset(); x.Amt__c=2; update x; r='ok'; } catch (Exception e) { r=e.getTypeName(); } System.assert(false, 'Fd|' + r); }
  @IsTest static void ptTest() { PtLog.reset(); PtP__c p = new PtP__c(Name='p'); insert p; insert new PtC__c(Name='a', P__c=p.Id, Amt__c=1); System.assert(false, 'pt|' + PtLog.seen); }
}
