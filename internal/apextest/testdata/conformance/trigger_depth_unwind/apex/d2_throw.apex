DpCtl.reset('throw');
DpA__c a = [SELECT Id FROM DpA__c LIMIT 1];
a.Name = 'p0';
try {
    update a;
    System.debug('OBS|throw|ok|d=' + DpCtl.depth);
} catch (Exception e) {
    String loc = DpCtl.loc(e);
    if (loc == 'none') {
        loc = e.getMessage().left(80);
    }
    System.debug('OBS|throw|' + e.getTypeName() + '|d=' + DpCtl.depth + '|loc=' + loc + '|max=' + e.getMessage().contains('maximum trigger depth exceeded'));
}
