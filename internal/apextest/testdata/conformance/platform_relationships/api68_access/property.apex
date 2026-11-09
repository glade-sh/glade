insert new Account(Name='Relationship probe');
Account a=[SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1];
System.debug(LoggingLevel.ERROR,'PROPERTY_ROW|'+String.valueOf(a.ContactPointOttSvcs.size()));
