Savepoint s=Database.setSavepoint();
insert new Account(Name='B6 Savepoint');
Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];
Reports.ReportManager.runReport(r.Id,false);
String result='ok';try {Database.rollback(s);}catch(Exception e){result=e.getTypeName()+'|'+e.getMessage();}
System.debug('B6SAVE|'+result);
