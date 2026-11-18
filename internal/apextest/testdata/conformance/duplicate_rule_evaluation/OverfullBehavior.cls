public class OverfullBehavior {
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
            l.add(acct(exact ? CN : 'Aldrovane Brightmor', city, '60601'));
        }
        return l;
    }
    public static String equal120() { return observe(accts(120, -1, -1, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String boundary99() { return observe(accts(99, 0, 98, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String boundary100() { return observe(accts(100, 0, 99, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String boundary101() { return observe(accts(101, 0, 100, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String strongLate() { return observe(accts(120, 100, 119, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String strongEarly() { return observe(accts(120, 0, 19, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String strongInterleaved() { return observe(accts(120, -1, -1, 6, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', null); }
    public static String touched() {
        List<Integer> t = new List<Integer>(); for (Integer i = 0; i < 20; i++) t.add(i);
        return observe(accts(120, 0, 119, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'insert', t);
    }
    static String twoPaths(Integer nameN, Integer webN) {
        List<SObject> l = new List<SObject>();
        Integer a = 0, b = 0;
        while (a < nameN || b < webN) {
            if (a < nameN && (b >= webN || a * webN <= b * nameN)) { l.add(acct(CN, 'Ovcity', '70001')); a++; }
            else { l.add(new Account(Name = 'Quillonde Farthing ' + String.valueOf(b), Website = 'https://tp.example', BillingPostalCode = '70002', Phone = '(630) 555-7719', BillingCity = 'Elsewhere')); b++; }
        }
        Account c = new Account(Name = CN, BillingCity = 'Ovcity', BillingPostalCode = '70002', Website = 'https://tp.example', Phone = '(630) 555-7719');
        return observe(l, c, 'insert', null);
    }
    public static String twoPaths60() { return twoPaths(60, 60); }
    public static String twoPaths110_10() { return twoPaths(110, 10); }
    public static String twoPaths10_110() { return twoPaths(10, 110); }
    public static String twoGates() {
        List<SObject> l = new List<SObject>();
        for (Integer i = 0; i < 120; i++) l.add(Math.mod(i, 2) == 0 ? acct(CN, 'Ovcity', '71001') : acct(CN, 'Othercity', '71002'));
        return observe(l, acct(CN, 'Ovcity', '71002'), 'insert', null);
    }
    public static String nameOrder() {
        List<SObject> l = new List<SObject>();
        for (Integer i = 0; i < 120; i++) { Integer r = 119 - i; l.add(acct('Aldrovane Brightmoor ' + String.fromCharArray(new List<Integer>{ 65 + r / 26 }) + String.fromCharArray(new List<Integer>{ 65 + Math.mod(r, 26) }), 'Ovcity', null)); }
        return observe(l, acct(CN, 'Ovcity', null), 'insert', null);
    }
    public static String byId() { return observe(accts(120, -1, -1, 0, 'Ovcity'), acct(CN, 'Ovcity', null), 'byid', null); }
    public static String contact120() {
        List<SObject> l = new List<SObject>();
        for (Integer i = 0; i < 120; i++) l.add(new Contact(FirstName = 'Dorvelle', LastName = 'Ostrander', Email = 'ov@ovx.example'));
        return observe(l, new Contact(FirstName = 'Dorvelle', LastName = 'Ostrander', Email = 'ov@ovx.example'), 'insert', null);
    }
    public static String lead120() {
        List<SObject> l = new List<SObject>();
        for (Integer i = 0; i < 120; i++) l.add(new Lead(FirstName = 'Dorvelle', LastName = 'Ostrander', Email = 'ov@ovx.example', Company = 'Ovco'));
        return observe(l, new Lead(FirstName = 'Dorvelle', LastName = 'Ostrander', Email = 'ov@ovx.example', Company = 'Ovco'), 'insert', null);
    }
}
