List<Parent__c> ps = new List<Parent__c>{new Parent__c(Name='omitted'), new Parent__c(Name='null', Flag__c=null), new Parent__c(Name='false', Flag__c=false), new Parent__c(Name='true', Flag__c=true)};
for (Parent__c p : ps) { System.debug('ROW|before|' + p.Name + '|' + p.Flag__c + '|' + p.get('Flag__c')); }
List<Parent__c> inserted = new List<Parent__c>();
for (Parent__c p : ps) { try { insert p; inserted.add(p); System.debug('ROW|insert|' + p.Name + '|success'); } catch (DmlException e) { System.debug('ROW|insert|' + p.Name + '|' + e.getDmlStatusCode(0)); } }
for (Parent__c p : ps) { System.debug('ROW|after|' + p.Name + '|' + p.Flag__c + '|' + p.get('Flag__c')); }
List<Child__c> cs = new List<Child__c>();
for (Parent__c p : inserted) cs.add(new Child__c(Name=p.Name, Parent__c=p.Id));
insert cs;
for (Child__c c : [SELECT Name, Parent__r.Flag__c FROM Child__c ORDER BY Name]) { SObject p = c.getSObject('Parent__r'); System.debug('ROW|soql|' + c.Name + '|' + c.Parent__r.Flag__c + '|' + p.get('Flag__c')); }
for (String kind : new List<String>{'null','false','true'}) { Parent__c p = new Parent__c(Name=kind); if (kind=='null') p.put('Flag__c',null); else p.put('Flag__c',kind=='true'); Child__c c = new Child__c(Parent__r=p); System.debug('ROW|linked-put|' + kind + '|' + c.Parent__r.Flag__c + '|' + c.getSObject('Parent__r').get('Flag__c')); }
for (Parent__c p : ps) { Child__c c = new Child__c(Parent__r=p); System.debug('ROW|linked-constructor|' + p.Name + '|' + c.Parent__r.Flag__c + '|' + c.getSObject('Parent__r').get('Flag__c')); }
try { Parent__c p = new Parent__c(); p.put('Flag__c', 'bad'); System.debug('ROW|reject|unexpected-success'); } catch (Exception e) { System.debug('ROW|reject|' + e.getTypeName()); }
try { SObject w = Schema.getGlobalDescribe().get('WebStore').newSObject(); w.put('IsActive',null); System.debug('ROW|webstore|' + w.get('IsActive')); } catch (Exception e) { System.debug('ROW|webstore-error|' + e.getTypeName() + '|' + e.getMessage()); }
