Integer ok = 0; String err = 'none'; Integer firstFail = -1;
for (Integer i = 0; i < 501; i++) {
    Database.SaveResult sr = EventBus.publish(new Tb_Evt__e(Note__c = 'e' + i));
    if (sr.isSuccess()) { ok++; } else if (err == 'none') { err = '' + sr.getErrors()[0].getStatusCode() + ':' + sr.getErrors()[0].getMessage(); firstFail = i; }
}
obs = ok + '|' + firstFail + '|' + err;
