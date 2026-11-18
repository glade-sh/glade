public class EditBehavior {
    static final List<String> STEMS = new List<String>{ 'Quendrel','Vexbury','Zandrik','Fenwraith','Corvelle','Marnotte','Plorindo','Quavasque','Ostrovane','Hirsanth','Kestrunn','Brinvask','Tavishon','Gilberta','Wexlarne','Dunthorpe','Pellimore','Rexvolt','Piquanto','Vantrosk','Wexmarrow','Karvanth','Mirvanth','Bromvaine','Thornquist','Jorvikand','Eldermere','Sundrovic','Calloway','Irvenholt','Yarbrelle','Nobelston','Gustavson','Hallowmere','Lindqvarn','Mortensky','Rosvenlad','Tilbrookan','Umberlane','Valdenmoor' };
    static final List<String> OTHERS = new List<String>{ 'Zxcvbnmq','Bwtfrdgh','Plkjnbvc','Xqzwsxed','Cfrtgbhn','Vgyhnjmk','Bhujmkol','Nmkjhgfd','Mlpokijn','Asdfqwer','Poiuytre','Lkjhgfds','Mnbvcxzl','Qazwsxed','Edcrfvtg','Rfvtgbyh','Tgbyhnuj','Yhnujmik','Ujmikolp','Ikolpmnb','Olpmnbvc','Plmnbvcx','Zaqxswcd','Xswcdevf','Cdevfrbg','Vfrbgtnh','Bgtnhymj','Nhymjukl','Mjuklopz','Kloplmxq','Wsxedcrf','Edcrfvtb','Rfvtbgyn','Tbgynhum','Ygbnhjuk','Unhjmkil','Imkilopp','Olpzxcvb','Pzxcvbnm','Qwertpas' };
    class Cfg { String key; String obj; List<String> mf; Boolean filt; String mode; }
    static Cfg cfg(String key) {
        Cfg c = new Cfg(); c.key = key; c.obj = 'Contact'; c.filt = false; c.mode = 'AND';
        if (key == 'R1') c.mf = new List<String>{ 'Phone' };
        else if (key == 'R2') c.mf = new List<String>{ 'Email' };
        else if (key == 'R3') c.mf = new List<String>{ 'FirstName', 'LastName' };
        else if (key == 'R4') c.mf = new List<String>{ 'Email', 'Phone' };
        else if (key == 'R5') { c.mf = new List<String>{ 'Phone' }; c.filt = true; }
        else if (key == 'R6') { c.mf = new List<String>{ 'Phone' }; c.filt = true; c.mode = 'OR'; }
        else if (key == 'R7') { c.mf = new List<String>{ 'Email' }; c.obj = 'Lead'; }
        return c;
    }
    static String val(String f, String k, Integer s) {
        if (f == 'Phone') {
            if (k == 'o') return '(' + String.valueOf(800 + s) + ') ' + String.valueOf(990 - 3 * s) + '-' + String.valueOf(6000 + s * 29);
            return '(' + String.valueOf(300 + 7 * s) + ') ' + String.valueOf(220 + 11 * s) + '-' + String.valueOf(1000 + 37 * s + (k == 'a' ? 1 : 0));
        }
        if (f == 'Email') return k == 'o' ? 'o' + s + '@n' + s + '.example' : 'e' + s + '@m' + s + '.example';
        if (f == 'FirstName') return k == 'm' ? 'Marlowen' : k == 'a' ? 'Marlowan' : 'Odelmar';
        return k == 'o' ? OTHERS[Math.mod(s, OTHERS.size())] : STEMS[Math.mod(s, STEMS.size())];
    }
    static Map<String,Object> vals(Cfg c, Integer s, String kind, Boolean inScope) {
        Map<String,Object> m = new Map<String,Object>();
        for (String f : c.mf) m.put(f, val(f, kind, s));
        if (!m.containsKey('LastName')) m.put('LastName', OTHERS[Math.mod(s + 7, OTHERS.size())] + 'z');
        if (c.obj == 'Lead') m.put('Company', 'Co' + s);
        if (c.filt) { m.put('Title', inScope ? 'XT' : 'nope'); m.put('Department', inScope ? 'YD' : 'nope'); }
        return m;
    }
    static SObject mk(Cfg c, Map<String,Object> m) {
        SObject o = c.obj == 'Lead' ? (SObject) new Lead() : (SObject) new Contact();
        for (String k : m.keySet()) o.put(k, m.get(k));
        return o;
    }
    static Database.DMLOptions allow() { Database.DMLOptions o = new Database.DMLOptions(); o.DuplicateRuleHeader.allowSave = true; return o; }
    // Create seed B (matching values, in scope) and A with the given values. Returns [B, A].
    static List<SObject> pair(Cfg c, Integer s, Map<String,Object> aVals) {
        SObject b = mk(c, vals(c, s, 'm', true)); SObject a = mk(c, aVals);
        Database.insert(new List<SObject>{ b, a }, allow());
        return new List<SObject>{ b, a };
    }
    static String res(Boolean ok, List<Database.Error> errs, Map<Id,String> labels) {
        if (ok) return 'S';
        Database.Error e = errs[0];
        if (e instanceof Database.DuplicateError) {
            Datacloud.DuplicateResult dr = ((Database.DuplicateError) e).getDuplicateResult();
            List<String> ms = new List<String>();
            for (Datacloud.MatchResult mr : dr.getMatchResults()) for (Datacloud.MatchRecord m : mr.getMatchRecords()) ms.add(labels.containsKey(m.getRecord().Id) ? labels.get(m.getRecord().Id) : 'other');
            ms.sort();
            return 'D' + (dr.isAllowSave() ? '+' : '-') + String.join(ms, '');
        }
        return 'E:' + String.valueOf(e.getStatusCode());
    }
    static String upd(SObject a, Map<String,Object> changes, Map<Id,String> labels) {
        SObject u = a.getSObjectType().newSObject(a.Id);
        for (String k : changes.keySet()) u.put(k, changes.get(k));
        Database.SaveResult r = Database.update(u, false);
        return res(r.isSuccess(), r.getErrors(), labels);
    }
    static Map<Id,String> lab(SObject b) { return new Map<Id,String>{ b.Id => 'B' }; }
    static Map<String,Object> setAll(Cfg c, Integer s, String kind) { Map<String,Object> m = new Map<String,Object>(); for (String f : c.mf) m.put(f, val(f, kind, s)); return m; }
    static Map<String,Object> nullAll(Cfg c) { Map<String,Object> m = new Map<String,Object>(); for (String f : c.mf) if (f != 'LastName') m.put(f, null); return m; }
    public static String edits(String key) {
        Cfg c = cfg(key);
        List<String> out = new List<String>();
        Integer s = 0;
        Map<String,Object> extra;
        // E0 unrelated field only, pair already matching
        s++; List<SObject> p = pair(c, s, vals(c, s, 'm', true));
        out.add('E0:' + upd(p[1], new Map<String,Object>{ 'Description' => 'unrelated' }, lab(p[0])));
        // E1 matching fields changed into a match
        s++; p = pair(c, s, vals(c, s, 'o', true));
        out.add('E1:' + upd(p[1], setAll(c, s, 'm'), lab(p[0])));
        // E2 assigned current value, pair already matching
        s++; p = pair(c, s, vals(c, s, 'm', true));
        out.add('E2:' + upd(p[1], setAll(c, s, 'm'), lab(p[0])));
        // E3 one of two matching fields changed, the other already equal
        if (c.mf.size() == 2) {
            s++; Map<String,Object> av = vals(c, s, 'm', true); String f2 = c.mf[1]; av.put(f2, val(f2, 'o', s));
            p = pair(c, s, av);
            out.add('E3:' + upd(p[1], new Map<String,Object>{ f2 => val(f2, key == 'R4' ? 'a' : 'm', s) }, lab(p[0])));
        } else out.add('E3:na');
        // E4 matching field to null, and from null
        s++; p = pair(c, s, vals(c, s, 'm', true));
        out.add('E4a:' + upd(p[1], nullAll(c), lab(p[0])));
        s++; Map<String,Object> nv = vals(c, s, 'o', true); for (String f : c.mf) if (f != 'LastName') nv.put(f, null);
        p = pair(c, s, nv);
        out.add('E4b:' + upd(p[1], setAll(c, s, 'm'), lab(p[0])));
        // E5 filter field enters scope, matching fields unchanged; E5b leaves scope
        if (c.filt) {
            s++; p = pair(c, s, vals(c, s, 'm', false));
            Map<String,Object> enter = new Map<String,Object>{ 'Title' => 'XT' };
            if (c.mode == 'AND') enter.put('Department', 'YD');
            out.add('E5:' + upd(p[1], enter, lab(p[0])));
            s++; p = pair(c, s, vals(c, s, 'm', true));
            out.add('E5b:' + upd(p[1], new Map<String,Object>{ 'Title' => 'nope', 'Department' => 'nope' }, lab(p[0])));
            s++; p = pair(c, s, vals(c, s, 'o', false));
            Map<String,Object> both = setAll(c, s, 'm'); both.putAll(enter);
            out.add('E6:' + upd(p[1], both, lab(p[0])));
        } else { out.add('E5:na'); out.add('E5b:na'); out.add('E6:na'); }
        // E7 before-update trigger rewrites the matching field
        s++; p = pair(c, s, vals(c, s, 'o', true));
        String f1 = c.mf[0];
        out.add('E7:' + upd(p[1], new Map<String,Object>{ 'Description' => 'rw|' + f1 + '|' + val(f1, 'm', s) }, lab(p[0])));
        // E8 changed to another matching value
        if (key != 'R2' && key != 'R7') {
            s++; p = pair(c, s, vals(c, s, 'm', true));
            Map<String,Object> alt = new Map<String,Object>(); for (String f : c.mf) alt.put(f, val(f, 'a', s));
            out.add('E8:' + upd(p[1], alt, lab(p[0])));
        } else out.add('E8:na');
        // E9 list update of two rows
        s++; List<SObject> p1 = pair(c, s, vals(c, s, 'o', true));
        s++; List<SObject> p2 = pair(c, s, vals(c, s, 'o', true));
        Map<Id,String> l2 = new Map<Id,String>{ p1[0].Id => 'B1', p2[0].Id => 'B2' };
        SObject u1 = p1[1].getSObjectType().newSObject(p1[1].Id), u2 = p2[1].getSObjectType().newSObject(p2[1].Id);
        for (String f : c.mf) { u1.put(f, val(f, 'm', s - 1)); u2.put(f, val(f, 'm', s)); }
        Database.SaveResult[] rl = Database.update(new List<SObject>{ u1, u2 }, false);
        out.add('E9:' + res(rl[0].isSuccess(), rl[0].getErrors(), l2) + ',' + res(rl[1].isSuccess(), rl[1].getErrors(), l2));
        // E10 upsert by Id, scalar and list
        s++; p = pair(c, s, vals(c, s, 'o', true));
        SObject ui = p[1].getSObjectType().newSObject(p[1].Id); for (String f : c.mf) ui.put(f, val(f, 'm', s));
        Database.UpsertResult ur = Database.upsert(ui, false);
        out.add('E10s:' + res(ur.isSuccess(), ur.getErrors(), lab(p[0])) + (ur.isSuccess() ? (ur.isCreated() ? '/created' : '/updated') : ''));
        s++; p = pair(c, s, vals(c, s, 'o', true)); s++; List<SObject> q = pair(c, s, vals(c, s, 'o', true));
        SObject ul1 = p[1].getSObjectType().newSObject(p[1].Id), ul2 = q[1].getSObjectType().newSObject(q[1].Id);
        for (String f : c.mf) { ul1.put(f, val(f, 'm', s - 1)); ul2.put(f, val(f, 'm', s)); }
        Database.UpsertResult[] url = Database.upsert(new List<SObject>{ ul1, ul2 }, false);
        Map<Id,String> l3 = new Map<Id,String>{ p[0].Id => 'B1', q[0].Id => 'B2' };
        out.add('E10l:' + res(url[0].isSuccess(), url[0].getErrors(), l3) + ',' + res(url[1].isSuccess(), url[1].getErrors(), l3));
        // E11 upsert by external key: new record into a match (insert path); existing keyed record changed into a match
        s++; SObject b11 = mk(c, vals(c, s, 'm', true)); Database.insert(b11, allow());
        SObject n11 = mk(c, vals(c, s, 'm', true)); n11.put('Ext__c', 'N' + s);
        Database.UpsertResult un = c.obj == 'Lead' ? Database.upsert((Lead) n11, Lead.Ext__c, false) : Database.upsert((Contact) n11, Contact.Ext__c, false);
        out.add('E11new:' + res(un.isSuccess(), un.getErrors(), lab(b11)) + (un.isSuccess() ? (un.isCreated() ? '/created' : '/updated') : ''));
        s++; Map<String,Object> kv = vals(c, s, 'o', true); kv.put('Ext__c', 'K' + s); List<SObject> p3 = pair(c, s, kv);
        SObject k11 = mk(c, vals(c, s, 'm', true)); k11.put('Ext__c', 'K' + s);
        Database.UpsertResult uk = c.obj == 'Lead' ? Database.upsert((Lead) k11, Lead.Ext__c, false) : Database.upsert((Contact) k11, Contact.Ext__c, false);
        out.add('E11existing:' + res(uk.isSuccess(), uk.getErrors(), lab(p3[0])) + (uk.isSuccess() ? (uk.isCreated() ? '/created' : '/updated') : ''));
        return key + '|' + String.join(out, '|');
    }
}
