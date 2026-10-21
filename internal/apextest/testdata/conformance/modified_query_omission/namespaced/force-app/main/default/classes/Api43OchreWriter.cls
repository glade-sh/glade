public with sharing class Api43OchreWriter {
 public static Api43OchreLedger__c apply(Api43OchreLedger__c r,String mode) {
  if(mode=='preinc') { Decimal changed=++r.Count__c; }
  else if(mode=='postinc') { Decimal changed=r.Count__c++; }
  else if(mode=='predec') { Decimal changed=--r.Count__c; }
  else if(mode=='postdec') { Decimal changed=r.Count__c--; }
  else if(mode=='add') { r.Amount__c+=3; }
  else if(mode=='same') { r.Name=r.Name; }
  else if(mode=='setun') { r.Note__c='z'; }
  else if(mode=='putun') { r.put('Note__c','z'); }
  else if(mode=='dml') { r.Amount__c+=3; update r; }
  else if(mode=='dmlonly') { update r; }
  else if(mode=='clone') { r.Amount__c+=3; return r.clone(true,true,false,true); }
  else if(mode=='cloneclean') { return r.clone(true,true,false,true); }
  else if(mode=='cloneAfter') { Api43OchreLedger__c c=r.clone(true,true,false,true); c.Amount__c+=3; return c; }
  return r;
 }
}