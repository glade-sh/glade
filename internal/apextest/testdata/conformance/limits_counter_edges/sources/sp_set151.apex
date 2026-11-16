for (Integer i = 0; i < 151; i++) { Database.setSavepoint(); }
obs = Limits.getDmlStatements() + '|' + Limits.getLimitDmlStatements() + '|' + Limits.getSavepoints() + '|' + Limits.getLimitSavepoints() + '|' + Limits.getSavepointRollbacks() + '|' + Limits.getLimitSavepointRollbacks() + '|' + Limits.getDmlRows();
