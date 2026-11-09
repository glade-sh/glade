Account parent = new Account(Name='C11 synthetic typeof'); insert parent;
Task t = new Task(Subject='C11 synthetic task',WhatId=parent.Id); insert t;
Event ev = new Event(Subject='C11 synthetic event',WhatId=parent.Id,StartDateTime=System.now(),EndDateTime=System.now().addHours(1)); insert ev;
Id taskId=t.Id; Id eventId=ev.Id;
List<SObject> tasks=Database.query('SELECT Id,TYPEOF What WHEN Account THEN Name END FROM Task WHERE Id=:taskId WITH USER_MODE');
List<SObject> events=Database.query('SELECT Id,TYPEOF What WHEN Account THEN Name END FROM Event WHERE Id=:eventId WITH USER_MODE');
Map<String,Object> out=new Map<String,Object>{'task_name'=>tasks[0].getSObject('What').get('Name'),'event_name'=>events[0].getSObject('What').get('Name')};
try{Database.query('SELECT Id,TYPEOF What WHEN User THEN Name END FROM Task WHERE Id=:taskId WITH USER_MODE');out.put('invalid_target','accepted');}catch(QueryException e){out.put('invalid_target',e.getTypeName());}
System.debug('C11CAPTURE|' + JSON.serialize(out));
