Account first = new Account(Name='NAV-TERNARY-A');
Account second = new Account(Name='NAV-TERNARY-B');
insert new List<Account>{first, second};
Boolean flag = true;
List<Account> rows = [SELECT Id FROM Account WHERE Name = :flag ? 'NAV-TERNARY-A' : 'NAV-TERNARY-B'];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:' + rows.size());
delete new List<Account>{first, second};
