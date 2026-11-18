List<Account> l = new List<Account>();
for (Integer i = 0; i < 10000; i++) { l.add(new Account(Name = 'DRINS' + i)); }
insert l;
obs = Limits.getDmlStatements() + '|' + Limits.getDmlRows() + '|' + Limits.getLimitDmlRows();
