Schema.FieldSet fs=Account.SObjectType.getDescribe().fieldSets.getMap().get('FamilyBaseline');
System.debug('S01|'+(fs.getNamespace()==null)+'|'+JSON.serialize(fs.getNamespace()));
List<String> refs=new List<String>(); for(Schema.SObjectType t:FamilyBaseline__c.OwnerId.getDescribe().getReferenceTo()) { refs.add(t.getDescribe().getName()); } System.debug('S02|'+JSON.serialize(refs));
String rel=Schema.getGlobalDescribe().get('FamilyBaseline__c').getDescribe().fields.getMap().get('Parent__c').getDescribe().getRelationshipName(); System.debug('J00|'+rel);
String first='{\"Id\":\"001000000000001AAA\",\"Name\":\"First\"}'; String second='{\"Id\":\"001000000000002AAA\",\"Name\":\"Second\"}';
try { SObject child=(SObject)JSON.deserialize('{"'+rel+'":'+first+',"'+rel.toLowerCase()+'":'+second+'}',FamilyBaseline__c.class); System.debug('J01|'+child.getSObject(rel).get('Name')+'|'+child.getSObject(rel.toLowerCase()).get('Name')); } catch(Exception e) { System.debug('J01|EXC|'+e.getTypeName()+'|'+e.getMessage()); }
try { SObject reverse=(SObject)JSON.deserialize('{"'+rel.toLowerCase()+'":'+second+',"'+rel+'":'+first+'}',FamilyBaseline__c.class); System.debug('J02|'+reverse.getSObject(rel).get('Name')+'|'+reverse.getSObject(rel.toLowerCase()).get('Name')); } catch(Exception e) { System.debug('J02|EXC|'+e.getTypeName()+'|'+e.getMessage()); }

Savepoint s=Database.setSavepoint(); Account a=new Account(Name='Single'); insert a; Id accountId=a.Id; SObject row=Database.query('SELECT Id, Name FROM Account WHERE Id=:accountId LIMIT 1'); System.debug('Q1|'+(a==(Account)row)+'|'+JSON.serialize(a.getPopulatedFieldsAsMap())+'|'+JSON.serialize(row.getPopulatedFieldsAsMap()));
System.debug('C1|'+a.get('LineCount__c')); Account q=Database.query('SELECT LineCount__c FROM Account WHERE Id=:accountId'); System.debug('C2|'+q.get('LineCount__c')); Database.rollback(s);
