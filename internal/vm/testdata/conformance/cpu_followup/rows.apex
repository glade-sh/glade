List<Account> rows = new List<Account>();
for (Integer i=0; i<1000; i++) rows.add(new Account(Name='CPU synthetic'));
Integer beforeInsert=Limits.getCpuTime();
insert rows;
Integer afterInsert=Limits.getCpuTime();
List<Account> found=[SELECT Id, Name FROM Account WHERE Name='CPU synthetic' LIMIT 1000];
Integer afterQuery=Limits.getCpuTime();
System.assert(false, 'insert_ms='+(afterInsert-beforeInsert)+', query_ms='+(afterQuery-afterInsert)+', rows='+found.size()+', total_ms='+afterQuery);
