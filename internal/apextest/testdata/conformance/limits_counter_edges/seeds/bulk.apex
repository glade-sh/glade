// repeat: 6
List<Account> l = new List<Account>();
for (Integer i = 0; i < 10000; i++) { l.add(new Account(Name = 'QRSEED__N__-' + i)); }
insert l;
