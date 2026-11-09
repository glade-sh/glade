List<Account> rows = [SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1]; System.debug(LoggingLevel.ERROR,'API68REL|ok');
