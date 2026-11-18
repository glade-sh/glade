String mode = 'agg76';
String ctx = 'f';
String jt = 'Future';
String cls = 'AsyncProbe__A__';
List<AsyncApexJob> js = [SELECT Status, ExtendedStatus FROM AsyncApexJob WHERE JobType = :jt AND ApexClass.Name = :cls ORDER BY CreatedDate DESC LIMIT 1];
if (js.isEmpty()) { obs = 'PENDING|nojob'; }
else if (js[0].Status != 'Completed' && js[0].Status != 'Failed' && js[0].Status != 'Aborted') { obs = 'PENDING|' + js[0].Status; }
else {
    List<Account> r = [SELECT Description FROM Account WHERE Name = :('AOBS-' + mode + '_' + ctx) LIMIT 1];
    if (r.isEmpty() && ctx == 'fin') { obs = 'PENDING|norec'; }
    else { obs = js[0].Status + '|' + js[0].ExtendedStatus + '|' + (r.isEmpty() ? 'NOREC' : r[0].Description); }
}
