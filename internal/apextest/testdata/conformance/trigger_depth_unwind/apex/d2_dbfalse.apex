DpCtl.reset('throw');
DpA__c a = [SELECT Id FROM DpA__c LIMIT 1];
List<Database.SaveResult> rs = Database.update(new List<DpA__c>{a}, false);
if (rs[0].isSuccess()) {
    System.debug('OBS|dbfalse|ok|d=' + DpCtl.depth);
} else {
    Database.Error er = rs[0].getErrors()[0];
    System.debug('OBS|dbfalse|' + er.getStatusCode() + '|loc=' + DpCtl.sloc(er.getMessage()) + '|d=' + DpCtl.depth);
}
