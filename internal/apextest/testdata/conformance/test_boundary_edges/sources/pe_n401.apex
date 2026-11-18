List<Tb_Evt__e> evs = new List<Tb_Evt__e>();
for (Integer i = 0; i < 401; i++) { evs.add(new Tb_Evt__e(Note__c = 'e' + i)); }
List<Database.SaveResult> rs = EventBus.publish(evs);
Integer ok = 0; String err = 'none';
for (Database.SaveResult sr : rs) { if (sr.isSuccess()) { ok++; } else if (err == 'none') { err = '' + sr.getErrors()[0].getStatusCode() + ':' + sr.getErrors()[0].getMessage(); } }
obs = rs.size() + '|' + ok + '|' + err;
