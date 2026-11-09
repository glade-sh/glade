DootSecQ5B1031__c parent=new DootSecQ5B1031__c(Name='C11 event custom'); insert parent;
Event ev=new Event(Subject='C11 event',WhatId=parent.Id,StartDateTime=System.now(),EndDateTime=System.now().addHours(1));insert ev;
Id eventId=ev.Id;
List<SObject> rows=Database.query('SELECT Id,TYPEOF What WHEN DootSecQ5B1031__c THEN Name END FROM Event WHERE Id=:eventId WITH USER_MODE');
Map<String,Object> out=new Map<String,Object>{'targetField'=>rows[0].getSObject('What').get('Name')};
try { Database.query('SELECT Id,TYPEOF What WHEN DootSecQ3B1031__c THEN Name END FROM Event WHERE Id=:eventId WITH USER_MODE');out.put('disabledTarget','accepted'); } catch(QueryException e) {out.put('disabledTarget',e.getTypeName());}
System.debug('C11CAPTURE|' + JSON.serialize(out));
