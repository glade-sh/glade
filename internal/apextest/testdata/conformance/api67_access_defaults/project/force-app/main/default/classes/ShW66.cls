public virtual with sharing class ShW66 {
  public virtual String cnt() { try { return 'n=' + [SELECT Id FROM Gl_Obj__c].size(); } catch (Exception e) { return GlFmt.sh(e); } }
  public virtual String dyn() { try { return 'n=' + Database.query('SELECT Id FROM Gl_Obj__c').size(); } catch (Exception e) { return GlFmt.sh(e); } }
  public virtual String sys() { try { return 'n=' + [SELECT Id FROM Gl_Obj__c WITH SYSTEM_MODE].size(); } catch (Exception e) { return GlFmt.sh(e); } }
  public virtual String sysDyn() { try { return 'n=' + Database.query('SELECT Id FROM Gl_Obj__c', AccessLevel.SYSTEM_MODE).size(); } catch (Exception e) { return GlFmt.sh(e); } }
  public virtual String upd(Id adm) { try { update new Gl_Obj__c(Id=adm, Open__c='u'); return 'OK'; } catch (Exception e) { return GlFmt.sh(e); } }
  public virtual String updSys(Id adm) { try { update as system new Gl_Obj__c(Id=adm, Open__c='u'); return 'OK'; } catch (Exception e) { return GlFmt.sh(e); } }
  public String all(Id adm) { return 'cnt=' + cnt() + ' dyn=' + dyn() + ' sys=' + sys() + ' sysDyn=' + sysDyn() + ' upd=' + upd(adm) + ' updSys=' + updSys(adm); }
}