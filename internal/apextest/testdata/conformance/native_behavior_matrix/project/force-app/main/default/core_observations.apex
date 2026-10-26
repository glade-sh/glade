System.debug('NULL_ITERATION');
try {
    List<String> items = null;
    for (String item : items) { System.debug(item); }
    System.debug('NULL_ITERATION|completed');
} catch (Exception ex) {
    System.debug('NULL_ITERATION|' + ex.getTypeName() + '|' + ex.getMessage());
}

System.debug('NULL_ELEMENT');
try {
    List<Account> rows = new List<Account>{null};
    System.debug(rows[0].Name);
    System.debug('NULL_ELEMENT|completed');
} catch (Exception ex) {
    System.debug('NULL_ELEMENT|' + ex.getTypeName() + '|' + ex.getMessage());
}

System.debug('LIST_INDEX');
try {
    List<String> empty = new List<String>();
    System.debug(empty[0]);
    System.debug('LIST_INDEX|completed');
} catch (Exception ex) {
    System.debug('LIST_INDEX|' + ex.getTypeName() + '|' + ex.getMessage());
}

try {
    System.debug('FEATURE_PARAMETER|' + FeatureManagement.checkPackageIntegerValue('MissingMatrixFeature'));
} catch (Exception ex) {
    System.debug('FEATURE_PARAMETER|' + ex.getTypeName() + '|' + ex.getMessage());
}

try {
    Account account = new Account(Name = 'Matrix Formula Record');
    insert account;
    List<Account> queried = [SELECT Id, Name FROM Account WHERE Id = :account.Id];
    List<SObject> records = new List<SObject>();
    records.addAll(queried);
    List<System.FormulaRecalcResult> recalculated = Formula.recalculateFormulas(records);
    System.debug('FORMULA_RECALCULATION|' + recalculated.size());
} catch (Exception ex) {
    System.debug('FORMULA_RECALCULATION|' + ex.getTypeName() + '|' + ex.getMessage());
}

Integer changedNowPairs = 0;
Integer changedMillisPairs = 0;
Long maxNowDelta = 0;
Long maxMillisDelta = 0;
for (Integer i = 0; i < 2000; i++) {
    Datetime firstNow = System.now();
    Datetime secondNow = System.now();
    Long nowDelta = secondNow.getTime() - firstNow.getTime();
    if (nowDelta != 0) changedNowPairs++;
    if (nowDelta > maxNowDelta) maxNowDelta = nowDelta;
    Long firstMillis = System.currentTimeMillis();
    Long secondMillis = System.currentTimeMillis();
    Long millisDelta = secondMillis - firstMillis;
    if (millisDelta != 0) changedMillisPairs++;
    if (millisDelta > maxMillisDelta) maxMillisDelta = millisDelta;
}
System.debug('CLOCK_TICKS|' + changedNowPairs + '|2000|' + maxNowDelta + '|' + changedMillisPairs + '|2000|' + maxMillisDelta);
