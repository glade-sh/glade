DpCtl.reset('catchlog');
DpA__c a = [SELECT Id FROM DpA__c LIMIT 1];
a.Name = 'p1';
try {
    update a;
    System.debug('OBS|catchlog|ok|d=' + DpCtl.depth);
} catch (Exception e) {
    System.debug('OBS|catchlog|' + e.getTypeName() + '|d=' + DpCtl.depth + '|loc=' + DpCtl.loc(e) + '|chain=' + DpCtl.log + '|L=' + DpCtl.counters);
}
