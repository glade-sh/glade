DpCtl.reset('swallow');
DpA__c a = [SELECT Id FROM DpA__c LIMIT 1];
a.Name = 'p2';
try {
    update a;
    System.debug('OBS|swallow|ok|d=' + DpCtl.depth + '|chain=' + DpCtl.log);
} catch (Exception e) {
    System.debug('OBS|swallow|' + e.getTypeName() + '|d=' + DpCtl.depth + '|chain=' + DpCtl.log);
}
