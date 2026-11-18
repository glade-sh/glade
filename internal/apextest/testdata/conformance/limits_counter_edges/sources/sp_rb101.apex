Savepoint sp = Database.setSavepoint();
Integer n = 0;
String first = 'none';
for (Integer i = 0; i < 101; i++) {
    try { Database.rollback(sp); n++; } catch (Exception e) { if (first == 'none') { first = i + ':' + e.getTypeName() + ':' + e.getMessage(); } }
}
obs = n + '|' + first + '|' + Limits.getDmlStatements() + '|' + Limits.getLimitDmlStatements() + '|' + Limits.getSavepoints() + '|' + Limits.getLimitSavepoints() + '|' + Limits.getSavepointRollbacks() + '|' + Limits.getLimitSavepointRollbacks() + '|' + Limits.getDmlRows();
