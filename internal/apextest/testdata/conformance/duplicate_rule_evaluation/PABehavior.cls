public class PABehavior {
    static Id prt() { return Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('PersonAccount').getRecordTypeId(); }
    static Id brt() { return Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('Business_Account').getRecordTypeId(); }
    static String nz(String s) { return s == '_' ? null : s; }
    static Account person(String spec, String key) {
        // first,last,email,phone,street,zip
        List<String> p = spec.split(',');
        return new Account(RecordTypeId = prt(), FirstName = nz(p[0]), LastName = nz(p[1]), PersonEmail = nz(p[2]), Phone = nz(p[3]), PersonMailingStreet = nz(p[4]), PersonMailingPostalCode = nz(p[5]));
    }
    static String hits(Datacloud.DuplicateResult rule, Map<Id,String> labels) {
        List<String> found = new List<String>();
        for (Datacloud.MatchResult result : rule.getMatchResults()) {
            for (Datacloud.MatchRecord match : result.getMatchRecords()) {
                Id mid = match.getRecord().Id;
                String l = labels.containsKey(mid) ? labels.get(mid) : 'unknown';
                Double raw = match.getMatchConfidence();
                found.add(l + '=' + String.valueOf(raw.intValue()) + '~' + String.valueOf(raw));
            }
        }
        found.sort();
        return found.isEmpty() ? '-' : String.join(found, '+');
    }
    static String findStr(List<Datacloud.FindDuplicatesResult> found, Integer i, Map<Id,String> labels) {
        List<String> parts = new List<String>();
        for (Datacloud.DuplicateResult dr : found[i].getDuplicateResults()) parts.add(dr.getDuplicateRule() + ':' + hits(dr, labels));
        return parts.isEmpty() ? 'norule' : String.join(parts, ';');
    }
    static String saveStr(Database.SaveResult sr, Map<Id,String> labels) {
        if (sr.isSuccess()) return 'S';
        Database.Error e = sr.getErrors()[0];
        if (e instanceof Database.DuplicateError) {
            Datacloud.DuplicateResult dr = ((Database.DuplicateError) e).getDuplicateResult();
            return 'D' + dr.getDuplicateRule() + ':' + hits(dr, labels) + (dr.isAllowSave() ? '' : '!');
        }
        return 'E:' + String.valueOf(e.getStatusCode()) + ':' + e.getMessage();
    }
    static String ex(Exception e) { return 'EX:' + e.getTypeName() + ':' + e.getMessage(); }
    static Database.DMLOptions allow() { Database.DMLOptions o = new Database.DMLOptions(); o.DuplicateRuleHeader.allowSave = true; return o; }
    static final List<String> PAIRS = new List<String>{
        'Marlowen,Quendrel,pa1@pk1.example,_,_,_/Marlowen,Quendrel,pa1@pk1.example,_,_,_',
        'Marlowen,Vexbury,pa2@pk2.example,_,_,_/Marlowan,Vexbury,pa2@pk2.example,_,_,_',
        'Marlowen,Zandrik,pa3@pk3.example,_,_,_/M,Zandrik,pa3@pk3.example,_,_,_',
        'Marlowen,Hirenokd,pa4@pk4.example,_,_,_/Marlowen,Hirenodk,pa4@pk4.example,_,_,_',
        'Odelmar,Kestrunn,_,(631) 555-4417,_,_/Hobart,Wexlarne,_,(631) 555-4417,_,_',
        'Odelmar,Brinvask,_,_,418 Ploverwick Lane,48192/Tavish,Corlenne,_,_,418 Ploverwick Lane,48192',
        'Odelmar,Fenwraith,pa7a@pk7.example,_,_,_/Gilberta,Ostrovane,pb7b@pq7.example,_,_,_',
        'Orzabalia,Tanagrolo,pa9@pk9.example,_,_,_/Orzabalia,Tanagrolo,pa9@pk9.example,_,_,_',
        'Wivelsky,Marnottes,pa10@pk10.example,_,_,_/Plorindo,Quavasque,pa10@pk10.example,_,_,_'
    };
    // paFields and paById share the pairs. mode: 'find' (by value + insert) or 'byid'
    static String panel(String mode) {
        List<Account> seeds = new List<Account>(), cands = new List<Account>();
        for (Integer i = 0; i < PAIRS.size(); i++) {
            List<String> pr = PAIRS[i].split('/');
            seeds.add(person(pr[0], String.valueOf(i))); cands.add(person(pr[1], String.valueOf(i)));
        }
        Database.insert(seeds, allow());
        Map<Id,String> labels = new Map<Id,String>();
        for (Integer i = 0; i < seeds.size(); i++) labels.put(seeds[i].Id, String.valueOf(i));
        List<String> out = new List<String>();
        if (mode == 'find') {
            List<Datacloud.FindDuplicatesResult> found;
            try { found = Datacloud.FindDuplicates.findDuplicates(cands); } catch (Exception e) { return ex(e); }
            Database.SaveResult[] sr = Database.insert(cands, false);
            for (Integer i = 0; i < cands.size(); i++) out.add(String.valueOf(i) + ':' + findStr(found, i, labels) + '/' + saveStr(sr[i], labels));
        } else {
            Database.SaveResult[] sr = Database.insert(cands, allow());
            List<Id> ids = new List<Id>();
            for (Account c : cands) ids.add(c.Id);
            List<Datacloud.FindDuplicatesResult> found;
            try { found = Datacloud.FindDuplicatesByIds.findDuplicatesByIds(ids); } catch (Exception e) { return ex(e); }
            for (Integer i = 0; i < cands.size(); i++) out.add(String.valueOf(i) + ':' + findStr(found, i, labels));
        }
        return String.join(out, ',');
    }
    public static String paFields() { return panel('find'); }
    public static String paById() { return panel('byid'); }
    public static String paEdit() {
        List<String> out = new List<String>();
        Account s1 = person('Odelmar,Rexvolt,ed1@ek1.example,_,_,_', '1'), e1 = person('Odelmar,Piquanto,ed1x@ek1.example,_,_,_', '1');
        Database.insert(new List<Account>{ s1, e1 }, allow());
        Map<Id,String> labels = new Map<Id,String>{ s1.Id => 's1' };
        Account u = new Account(Id = e1.Id, LastName = 'Rexvolt', PersonEmail = 'ed1@ek1.example');
        out.add('updateIntoMatch=' + saveStr(Database.update(u, false), labels));
        Account s2 = person('Cybrella,Dunthorpe,ed2@ek2.example,_,_,_', '2');
        Account u2 = person('Cybrella,Dunthorpe,ed2@ek2.example,_,_,_', '2'); u2.Ext__c = 'PX1';
        Database.insert(s2, allow());
        Database.UpsertResult ur = Database.upsert(u2, Account.Ext__c, false);
        String up = 'S';
        if (!ur.isSuccess()) { Database.Error e = ur.getErrors()[0]; up = e instanceof Database.DuplicateError ? 'D' + ((Database.DuplicateError) e).getDuplicateResult().getDuplicateRule() : 'E:' + String.valueOf(e.getStatusCode()); }
        out.add('upsertScalar=' + up);
        Account a = person('Garnet,Pellimore,ed3@ek3.example,_,_,_', '3'), b = person('Garnet,Pellimore,ed3@ek3.example,_,_,_', '3');
        Database.insert(new List<Account>{ a, b }, allow());
        Database.SaveResult d1 = Database.update(new Account(Id = a.Id, Description = 'edited'), false);
        out.add('descriptionOnly=' + saveStr(d1, new Map<Id,String>{ b.Id => 'pair' }));
        return String.join(out, '|');
    }
    public static String crossSubtype() {
        List<String> out = new List<String>();
        Account ps = person('Thessaly,Bromvaine,cs1@cx1.example,_,_,_', '1'); ps.PersonMailingCity = 'Cxcity';
        Account bs = new Account(RecordTypeId = brt(), Name = 'Thessaly Quorrimand', BillingCity = 'Cxcity');
        Database.insert(new List<Account>{ ps, bs }, allow());
        Map<Id,String> labels = new Map<Id,String>{ ps.Id => 'person', bs.Id => 'business' };
        Account pc = person('Thessaly,Quorrimand,cs2@cx2.example,_,_,_', '2'); pc.PersonMailingCity = 'Cxcity';
        Account bc = new Account(RecordTypeId = brt(), Name = 'Thessaly Bromvaine', BillingCity = 'Cxcity');
        try {
            List<Datacloud.FindDuplicatesResult> f = Datacloud.FindDuplicates.findDuplicates(new List<Account>{ pc });
            out.add('personVsBusinessSeed.find=' + findStr(f, 0, labels));
        } catch (Exception e) { out.add('personVsBusinessSeed.find=' + ex(e)); }
        out.add('personVsBusinessSeed.insert=' + saveStr(Database.insert(pc, false), labels));
        try {
            List<Datacloud.FindDuplicatesResult> f = Datacloud.FindDuplicates.findDuplicates(new List<Account>{ bc });
            out.add('businessVsPersonSeed.find=' + findStr(f, 0, labels));
        } catch (Exception e) { out.add('businessVsPersonSeed.find=' + ex(e)); }
        out.add('businessVsPersonSeed.insert=' + saveStr(Database.insert(bc, false), labels));
        return String.join(out, '|');
    }
    public static String noRecordType() {
        List<String> out = new List<String>();
        Account seed = person('Ysolde,Karvanth,nr1@nx1.example,_,_,_', '1');
        Database.insert(seed, allow());
        Map<Id,String> labels = new Map<Id,String>{ seed.Id => 'seed' };
        Account noRt = new Account(FirstName = 'Ysolde', LastName = 'Karvanth', PersonEmail = 'nr1@nx1.example');
        try { out.add('noRt.find=' + findStr(Datacloud.FindDuplicates.findDuplicates(new List<Account>{ noRt }), 0, labels)); } catch (Exception e) { out.add('noRt.find=' + ex(e)); }
        try { out.add('noRt.insert=' + saveStr(Database.insert(noRt, false), labels)); } catch (Exception e) { out.add('noRt.insert=' + ex(e)); }
        Account bus = new Account(RecordTypeId = brt(), Name = 'Ysolde Karvanth');
        try { out.add('business.find=' + findStr(Datacloud.FindDuplicates.findDuplicates(new List<Account>{ bus }), 0, labels)); } catch (Exception e) { out.add('business.find=' + ex(e)); }
        return String.join(out, '|');
    }
    public static String mixedList() {
        List<String> out = new List<String>();
        Account seed = person('Ysolde,Mirvanth,ml1@mx1.example,_,_,_', '1');
        Account bseed = new Account(RecordTypeId = brt(), Name = 'Mixedco Larnavore', BillingCity = 'Mxcity');
        Database.insert(new List<Account>{ seed, bseed }, allow());
        Map<Id,String> labels = new Map<Id,String>{ seed.Id => 'person', bseed.Id => 'business' };
        Account pc = person('Ysolde,Mirvanth,ml1@mx1.example,_,_,_', '1');
        Account bc = new Account(RecordTypeId = brt(), Name = 'Mixedco Larnavore', BillingCity = 'Mxcity');
        for (Integer order = 0; order < 2; order++) {
            List<Account> l = order == 0 ? new List<Account>{ bc, pc } : new List<Account>{ pc, bc };
            try {
                List<Datacloud.FindDuplicatesResult> f = Datacloud.FindDuplicates.findDuplicates(l);
                List<String> r = new List<String>();
                for (Integer i = 0; i < f.size(); i++) r.add(findStr(f, i, labels));
                out.add('order' + order + '=' + String.join(r, ';;'));
            } catch (Exception e) { out.add('order' + order + '=' + ex(e)); }
        }
        return String.join(out, '|');
    }
    public static String byIdBatch() {
        List<String> out = new List<String>();
        List<Account> ps = new List<Account>();
        for (Integer i = 0; i < 51; i++) ps.add(new Account(RecordTypeId = prt(), FirstName = 'Batchy', LastName = 'Pers' + String.valueOf(1000 + i * 7) + 'qz', PersonEmail = 'bb' + i + '@bx.example'));
        Database.insert(ps, allow());
        Account b = new Account(RecordTypeId = brt(), Name = 'Batchco Zantrel');
        insert b;
        List<Id> ids50 = new List<Id>(), ids51 = new List<Id>(), mix = new List<Id>();
        for (Integer i = 0; i < 51; i++) { if (i < 50) ids50.add(ps[i].Id); ids51.add(ps[i].Id); }
        mix.add(ps[0].Id); mix.add(b.Id); mix.add(ps[1].Id);
        for (String k : new List<String>{ 'ids50', 'ids51', 'mix' }) {
            try {
                List<Datacloud.FindDuplicatesResult> f = Datacloud.FindDuplicatesByIds.findDuplicatesByIds(k == 'ids50' ? ids50 : k == 'ids51' ? ids51 : mix);
                out.add(k + '=ok size=' + String.valueOf(f.size()));
            } catch (Exception e) { out.add(k + '=' + ex(e)); }
        }
        return String.join(out, '|');
    }
    public static String twoRules() {
        List<String> out = new List<String>();
        Account seed = person('Cordelle,Vantrosk,tr1@tx1.example,_,_,_', '1');
        Database.insert(seed, allow());
        Map<Id,String> labels = new Map<Id,String>{ seed.Id => 'seed' };
        Account c = person('Cordelle,Vantrosk,tr1@tx1.example,_,_,_', '1');
        try {
            List<Datacloud.FindDuplicatesResult> f = Datacloud.FindDuplicates.findDuplicates(new List<Account>{ c });
            out.add('find=' + findStr(f, 0, labels));
        } catch (Exception e) { out.add('find=' + ex(e)); }
        Account c2 = c.clone(false);
        out.add('insertDefault=' + saveStr(Database.insert(c2, false), labels));
        Account c3 = c.clone(false);
        Database.SaveResult o = Database.insert(c3, allow());
        out.add('insertOverride=' + (o.isSuccess() ? 'S' : saveStr(o, labels)));
        return String.join(out, '|');
    }
    public static String resultShape() {
        List<String> out = new List<String>();
        Account seed = person('Cordelle,Wexmarrow,rs1@rx1.example,_,_,_', '1');
        Database.insert(seed, allow());
        Account c = person('Cordelle,Wexmarrow,rs1@rx1.example,_,_,_', '1');
        List<Datacloud.FindDuplicatesResult> f = Datacloud.FindDuplicates.findDuplicates(new List<Account>{ c });
        out.add('results=' + String.valueOf(f.size()));
        for (Datacloud.DuplicateResult dr : f[0].getDuplicateResults()) {
            out.add('rule=' + dr.getDuplicateRule() + ',allowSave=' + dr.isAllowSave() + ',error=' + dr.getErrorMessage());
            for (Datacloud.MatchResult mr : dr.getMatchResults()) {
                out.add('entity=' + mr.getEntityType() + ',engine=' + mr.getMatchEngine() + ',matchrule=' + mr.getRule() + ',size=' + String.valueOf(mr.getSize()) + ',success=' + mr.isSuccess());
            }
        }
        return String.join(out, '|');
    }
}
