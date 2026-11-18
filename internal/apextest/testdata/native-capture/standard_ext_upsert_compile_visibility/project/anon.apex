Savepoint sp=Database.setSavepoint(); String r;
try {Account x=new Account(Name='A36',A36External__c='one');Database.UpsertResult s=Database.upsert(x,Account.A36External__c,true); r='ok '+s.isCreated();} catch(DmlException e){r='DML|'+e.getDmlStatusCode(0);} catch(Exception e){r='EXC|'+e.getTypeName()+'|'+e.getMessage();} finally {Database.rollback(sp);}
System.debug(LoggingLevel.ERROR,'PROBE '+r);
try {Account x=new Account(Name='A36',A36External__c='one');insert x; r='ins ok';} catch(Exception e){r='EXC|'+e.getTypeName()+'|'+e.getMessage();} 
System.debug(LoggingLevel.ERROR,'PROBE2 '+r);
