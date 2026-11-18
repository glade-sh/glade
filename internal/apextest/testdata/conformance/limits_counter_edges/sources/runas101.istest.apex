User u = [SELECT Id FROM User WHERE Id = :UserInfo.getUserId()];
for (Integer i = 0; i < 101; i++) { System.runAs(u) { Integer x = 1; } }
obs = Limits.getDmlStatements() + '|' + Limits.getLimitDmlStatements() + '|' + Limits.getSavepoints() + '|' + Limits.getLimitSavepoints() + '|' + Limits.getSavepointRollbacks() + '|' + Limits.getLimitSavepointRollbacks() + '|' + Limits.getDmlRows() + '|' + Limits.getRunAs() + '|' + Limits.getLimitRunAs();
