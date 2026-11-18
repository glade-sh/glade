public class AccountGateBehavior {
    private static String hits(Datacloud.DuplicateResult rule, Map<Id,String> labels) {
        List<String> found = new List<String>();
        for (Datacloud.MatchResult result : rule.getMatchResults()) {
            for (Datacloud.MatchRecord match : result.getMatchRecords()) {
                Id matchedId = match.getRecord().Id;
                System.assert(labels.containsKey(matchedId), 'unidentified seed');
                Double raw = match.getMatchConfidence();
                found.add(labels.get(matchedId) + '=' + String.valueOf(raw.intValue()) + '~' + String.valueOf(raw));
            }
        }
        found.sort();
        return found.isEmpty() ? '-' : String.join(found, '+');
    }
    private static String nz(String s) { return s == '_' ? null : s; }
    private static Account mk(String name, String web, String street, String city, String zip, String phone) {
        return new Account(Name = name, Website = nz(web), BillingStreet = nz(street), BillingCity = nz(city), BillingPostalCode = nz(zip), Phone = nz(phone));
    }
    public static String run(List<String> specs) {
        String ruleName = 'Standard_Account_Duplicate_Rule';
        List<String> all = new List<String>(specs);
        all.add('Zorbexa Tanagro~_~_~Ctpos~_~_~Zorbexa Tanagro~_~_~Ctpos~_~_~insert');
        all.add('Wivelsk Marnotte~_~_~Ctneg~_~_~Plorind Quavasque~_~_~Ctneg~_~_~insert');
        List<Account> seeds = new List<Account>(), cands = new List<Account>(), edits = new List<Account>();
        List<String> ops = new List<String>();
        for (Integer i = 0; i < all.size(); i++) {
            List<String> p = all[i].split('~');
            seeds.add(mk(p[0], p[1], p[2], p[3], p[4], p[5]));
            cands.add(mk(p[6], p[7], p[8], p[9], p[10], p[11]));
            ops.add(p[12]);
            if (p[12] != 'insert') edits.add(new Account(Name = 'Edit' + String.valueOf(i) + 'Zq', Website = 'https://edit' + String.valueOf(i) + '.invalid', BillingPostalCode = '9' + String.valueOf(1000 + i)));
        }
        insert seeds;
        if (!edits.isEmpty()) insert edits;
        Map<Id,String> labels = new Map<Id,String>();
        for (Integer i = 0; i < seeds.size(); i++) labels.put(seeds[i].Id, String.valueOf(i));
        List<Datacloud.FindDuplicatesResult> found = Datacloud.FindDuplicates.findDuplicates(cands);
        System.assertEquals(all.size(), found.size(), 'incomplete Find response');
        List<Database.SaveResult> saved = new List<Database.SaveResult>();
        Integer ei = 0;
        List<Account> insertList = new List<Account>();
        Map<Integer,Integer> idx = new Map<Integer,Integer>();
        Database.SaveResult[] insertResults = new Database.SaveResult[0];
        List<Integer> insertIdx = new List<Integer>();
        Map<Integer,Database.SaveResult> res = new Map<Integer,Database.SaveResult>();
        for (Integer i = 0; i < all.size(); i++) {
            if (ops[i] == 'insert') { insertList.add(cands[i]); insertIdx.add(i); }
        }
        if (!insertList.isEmpty()) {
            Database.SaveResult[] r = Database.insert(insertList, false);
            for (Integer j = 0; j < r.size(); j++) res.put(insertIdx[j], r[j]);
        }
        for (Integer i = 0; i < all.size(); i++) {
            if (ops[i] == 'update') {
                Account u = cands[i].clone(false); u.Id = edits[ei++].Id;
                res.put(i, Database.update(u, false));
            } else if (ops[i] == 'upsert') {
                Account u = cands[i].clone(false); u.Ext__c = 'X' + String.valueOf(i);
                Database.UpsertResult ur = Database.upsert(u, Account.Ext__c, false);
                res.put(i, null);
                ei++;
                if (ur.isSuccess()) { outcomesU.put(i, 'S'); }
                else {
                    Database.Error[] errs = ur.getErrors();
                    String d = String.valueOf(errs[0].getStatusCode());
                    if (errs[0] instanceof Database.DuplicateError) { Datacloud.DuplicateResult dr = ((Database.DuplicateError) errs[0]).getDuplicateResult(); d = hits(dr, labels) + (dr.isAllowSave() ? '' : '!'); }
                    outcomesU.put(i, d);
                }
            }
        }
        List<String> outcomes = new List<String>();
        Integer savedCount = 0;
        for (Integer i = 0; i < all.size(); i++) {
            List<Datacloud.DuplicateResult> rules = found[i].getDuplicateResults();
            String f = '-';
            if (rules.size() == 1) { System.assertEquals(ruleName, rules[0].getDuplicateRule()); f = hits(rules[0], labels); }
            else f = 'rules' + String.valueOf(rules.size());
            String d = 'S';
            if (ops[i] == 'upsert') d = outcomesU.get(i);
            else {
                Database.SaveResult sr = res.get(i);
                if (sr.isSuccess()) d = 'S';
                else {
                    Database.Error[] errs = sr.getErrors();
                    String code = String.valueOf(errs[0].getStatusCode());
                    if (errs[0] instanceof Database.DuplicateError) { Datacloud.DuplicateResult dr = ((Database.DuplicateError) errs[0]).getDuplicateResult(); System.assertEquals(ruleName, dr.getDuplicateRule()); d = hits(dr, labels) + (dr.isAllowSave() ? '' : '!'); }
                    else d = 'E:' + code;
                }
            }
            if (d == 'S') savedCount++;
            outcomes.add(String.valueOf(i) + ':' + f + '/' + d);
        }
        System.assert(outcomes[outcomes.size() - 2].contains('='), 'positive control absent');
        System.assert(outcomes[outcomes.size() - 1].endsWith(':-/S'), 'negative control matched');
        return ruleName + '|saved=' + String.valueOf(savedCount) + '|' + String.join(outcomes, ',');
    }
    private static Map<Integer,String> outcomesU = new Map<Integer,String>();
    public static String m_gateOff2() {
        return run(new List<String>{
            'Gorvenwick Partners 0x~https://gw0.example~418 Larkmoor Avenue~Vandermeulen~50101~(628) 555-0140~Gorvenwick Partners 0xq~https://gw0.example~418 Larkmoor Avenue~Qorrisk~60202~(628) 555-0140~insert',
            'Gorvenwick Partners 1x~https://gw1.example~418 Larkmoor Avenue~Tabrinth~51101~(628) 555-0141~Gorvenwick Partners 1xq~https://gw1.example~418 Larkmoor Avenue~Zolvecka~61202~(628) 555-0141~insert',
            'Gorvenwick Partners 2x~https://gw2.example~418 Larkmoor Avenue~Hexamoor~52101~(628) 555-0142~Gorvenwick Partners 2xq~https://gw2.example~418 Larkmoor Avenue~Pruvendal~62202~(628) 555-0142~insert',
            'Gorvenwick Partners 3x~https://gw3.example~418 Larkmoor Avenue~Olinthra~53101~(628) 555-0143~Gorvenwick Partners 3xq~https://gw3.example~418 Larkmoor Avenue~Wekkesund~63202~(628) 555-0143~insert',
            'Gorvenwick Partners 4x~https://gw4.example~418 Larkmoor Avenue~Brasmere~54101~(628) 555-0144~Gorvenwick Partners 4xq~https://gw4.example~418 Larkmoor Avenue~Jotunvik~64202~(628) 555-0144~insert',
            'Gorvenwick Partners 5x~https://gw5.example~418 Larkmoor Avenue~Calderwyn~55101~(628) 555-0145~Gorvenwick Partners 5xq~https://gw5.example~418 Larkmoor Avenue~Xillamore~65202~(628) 555-0145~insert'
        });
    }
}
