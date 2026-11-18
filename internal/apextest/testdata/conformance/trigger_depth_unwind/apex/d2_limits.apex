DpCtl.reset('throw');
DpA__c a = [SELECT Id FROM DpA__c LIMIT 1];
try {
    update a;
    System.debug('OBS|limits|ok|d=' + DpCtl.depth);
} catch (Exception e) {
    System.debug('OBS|limits|' + e.getTypeName() + '|d=' + DpCtl.depth + '|dml=' + Limits.getDmlStatements() + '|rows=' + Limits.getDmlRows() + '|q=' + Limits.getQueries() + '|qr=' + Limits.getQueryRows());
}
