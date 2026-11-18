public class OverfullWeakBehavior {
    static final String CN = 'Aldrovane Brightmoor';
    static String comp(List<Integer> xs) {
        xs.sort();
        List<String> out = new List<String>();
        Integer i = 0;
        while (i < xs.size()) {
            Integer j = i;
            while (j + 1 < xs.size() && xs[j + 1] == xs[j] + 1) j++;
            out.add(j == i ? String.valueOf(xs[i]) : String.valueOf(xs[i]) + '-' + String.valueOf(xs[j]));
            i = j + 1;
        }
        return String.join(out, '.');
    }
    static String shape(Datacloud.DuplicateResult dr, Map<Id,Integer> labels) {
        List<Integer> idl = new List<Integer>();
        List<String> first = new List<String>();
        Map<String,Integer> confs = new Map<String,Integer>();
        Integer n = 0;
        for (Datacloud.MatchResult mr : dr.getMatchResults()) {
            for (Datacloud.MatchRecord m : mr.getMatchRecords()) {
                Integer l = labels.get(m.getRecord().Id);
                idl.add(l);
                Double raw = m.getMatchConfidence();
                String c = String.valueOf(raw.intValue());
                confs.put(c, (confs.containsKey(c) ? confs.get(c) : 0) + 1);
                if (n < 25) first.add(String.valueOf(l) + '=' + c);
                n++;
            }
        }
        List<String> cl = new List<String>();
        for (String k : new List<String>(confs.keySet())) cl.add(k + 'x' + String.valueOf(confs.get(k)));
        cl.sort();
        return 'size=' + String.valueOf(n) + ';order25=' + String.join(first, ',') + ';set=' + comp(idl) + ';conf=' + String.join(cl, ',') + ';rules=' + dr.getDuplicateRule();
    }
    // seeds: list of prepared records (SObject). candidate: prepared. mode: insert|byid
    static String observe(List<SObject> seeds, SObject cand, String mode, List<Integer> touch) {
        if (seeds.size() > 0 && cand.getSObjectType() == Account.SObjectType) { }
        Database.DMLOptions allow = new Database.DMLOptions(); allow.DuplicateRuleHeader.allowSave = true;
        Database.insert(seeds, allow);
        if (touch != null) {
            List<SObject> ups = new List<SObject>();
            for (Integer t : touch) { SObject s = seeds[t].getSObjectType().newSObject(seeds[t].Id); s.put('Description', 'touched'); ups.add(s); }
            Database.update(ups, allow);
        }
        Map<Id,Integer> labels = new Map<Id,Integer>();
        for (Integer i = 0; i < seeds.size(); i++) labels.put(seeds[i].Id, i);
        List<String> out = new List<String>();
        List<Datacloud.FindDuplicatesResult> f;
        if (mode == 'byid') {
            Database.insert(cand, allow);
            f = Datacloud.FindDuplicatesByIds.findDuplicatesByIds(new List<Id>{ cand.Id });
        } else {
            f = Datacloud.FindDuplicates.findDuplicates(new List<SObject>{ cand });
        }
        List<Datacloud.DuplicateResult> rs = f[0].getDuplicateResults();
        out.add('find:' + (rs.isEmpty() ? 'norule' : shape(rs[0], labels) + ';nrules=' + String.valueOf(rs.size())));
        if (mode != 'byid') {
            Database.SaveResult sr = Database.insert(cand, false);
            if (sr.isSuccess()) out.add('dml:S');
            else {
                Database.Error e = sr.getErrors()[0];
                if (e instanceof Database.DuplicateError) out.add('dml:' + shape(((Database.DuplicateError) e).getDuplicateResult(), labels));
                else out.add('dml:E:' + e.getMessage());
            }
        }
        return String.join(out, '|');
    }
    static Account acct(String name, String city, String zip) { return new Account(Name = name, BillingCity = city, BillingPostalCode = zip); }
    static String variant(Integer i) { return 'Aldrovane Brightmoor' + (i < 26 ? ' ' + String.fromCharArray(new List<Integer>{ 65 + i }) : ' ' + String.fromCharArray(new List<Integer>{ 65 + Math.mod(i, 26) }) + String.fromCharArray(new List<Integer>{ 97 + Math.mod(i / 26, 26) })); }
    static List<SObject> accts(Integer n, Integer exactFrom, Integer exactTo, Integer every, String city) {
        List<SObject> l = new List<SObject>();
        for (Integer i = 0; i < n; i++) {
            Boolean exact = (exactFrom >= 0 && i >= exactFrom && i <= exactTo) || (every > 0 && Math.mod(i, every) == 0 && exactFrom < 0);
            l.add(acct(exact ? CN : 'Aldrovane Brightmoor Zq', city, '60601'));
        }
        return l;
    }
    public static String strongLateWeak() { return observe(accts(120, 100, 119, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String strongEarlyWeak() { return observe(accts(120, 0, 19, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String strongInterleavedWeak() { return observe(accts(120, -1, -1, 6, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String strongLate110() { return observe(accts(130, 110, 129, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String weakOnly120() { return observe(accts(120, -1, -1, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null).substring(0); }
}
