String r = '';
for (Integer n : new List<Integer>{ 100, 100 }) {
    List<Tb_Evt__e> evs = new List<Tb_Evt__e>();
    for (Integer i = 0; i < n; i++) { evs.add(new Tb_Evt__e(Note__c = 'e' + i)); }
    List<Database.SaveResult> rs = EventBus.publish(evs);
    Integer ok = 0;
    for (Database.SaveResult sr : rs) { if (sr.isSuccess()) { ok++; } }
    r += n + ':' + ok + ';';
}
obs = r;
