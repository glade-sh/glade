for (Integer i = 0; i < 75; i++) { Savepoint sp = Database.setSavepoint(); Database.rollback(sp); }
obs = Limits.getDmlStatements() + '|' + Limits.getLimitDmlStatements() + '|' + Limits.getSavepoints() + '|' + Limits.getLimitSavepoints() + '|' + Limits.getSavepointRollbacks() + '|' + Limits.getLimitSavepointRollbacks() + '|' + Limits.getDmlRows();
