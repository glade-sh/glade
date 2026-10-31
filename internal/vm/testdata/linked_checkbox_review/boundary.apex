Parent__c p=new Parent__c(Name='parent',Flag__c=null,FalseFlag__c=null);
Child__c c=new Child__c(Parent__r=p);
System.debug('ROW|custom-null-true|'+c.Parent__r.Flag__c+'|'+p.get('Flag__c'));
System.debug('ROW|custom-null-false|'+c.Parent__r.FalseFlag__c+'|'+p.get('FalseFlag__c'));
Contact ct=new Contact(LastName='control',ContactFlag__c=null,DoNotCall=null);
System.debug('ROW|contact-custom-null|'+ct.ContactFlag__c+'|'+ct.get('ContactFlag__c'));
System.debug('ROW|contact-standard-null|'+ct.DoNotCall+'|'+ct.get('DoNotCall'));
try { SObject w=Schema.getGlobalDescribe().get('WebStore').newSObject(); w.put('OptionsAutoFacetingEnabled',null); System.debug('ROW|webstore-null|'+w.get('OptionsAutoFacetingEnabled')); } catch(Exception e) { System.debug('ROW|webstore-unavailable|'+e.getTypeName()); }
Parent__c fresh=new Parent__c(Name='inserted'); insert fresh;
System.debug('ROW|caller-omitted|'+fresh.Flag__c+'|'+fresh.get('Flag__c'));
Parent__c idOnly=new Parent__c(Id=fresh.Id);
System.debug('ROW|id-only|'+idOnly.Flag__c+'|'+idOnly.get('Flag__c'));
Parent__c queried=[SELECT Flag__c FROM Parent__c WHERE Id=:fresh.Id];
System.debug('ROW|queried|'+queried.Flag__c+'|'+queried.get('Flag__c'));
Parent__c clone=(Parent__c)fresh.clone(true);
System.debug('ROW|clone|'+clone.Flag__c+'|'+clone.get('Flag__c'));
try { Parent__c bad=new Parent__c(Name='bad',Flag__c=null); insert bad; System.debug('ROW|null-insert|success'); } catch(DmlException e) { System.debug('ROW|null-insert|'+e.getDmlStatusCode(0)); }
