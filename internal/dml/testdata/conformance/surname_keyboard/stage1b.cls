public class SurnameBehavior {
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
    private static SObject build(String obj, String first, String last, String email, String phone, String key) {
        SObject s = obj == 'Lead' ? (SObject) new Lead() : (SObject) new Contact();
        s.put('FirstName', first);
        s.put('LastName', last);
        if (email != null) s.put('Email', email);
        if (phone != null) s.put('Phone', phone);
        if (obj == 'Lead') s.put('Company', 'Cmp' + key);
        return s;
    }
    public static String run(String obj, Integer startKey, List<String> specs) {
        String ruleName = 'Standard_' + obj + '_Duplicate_Rule';
        List<String> all = new List<String>(specs);
        all.add('Orzabal/Orzabal/std');
        all.add('Zimbalist/Okonkwo/std');
        List<SObject> seeds = (List<SObject>) Type.forName('List<' + obj + '>').newInstance();
        List<SObject> candidates = (List<SObject>) Type.forName('List<' + obj + '>').newInstance();
        for (Integer i = 0; i < all.size(); i++) {
            List<String> p = all[i].split('/');
            String v = p[2];
            String key = String.valueOf(startKey + i);
            String email = 's' + key + '@k' + key + '.example';
            String phone = '(630) 555-' + String.valueOf(1000 + Math.mod(startKey + i, 9000));
            String sf = 'Brannock', cf = 'Brannock';
            String se = email, ce = email, sp = null, cp = null;
            if (v == 'noemail') { se = null; ce = null; }
            else if (v == 'diffemail') { ce = 'c' + key + '@z' + key + '.example'; }
            else if (v == 'diffname') { cf = 'Ormerod'; }
            else if (v == 'phone') { se = null; ce = null; sp = phone; cp = phone; }
            else System.assertEquals('std', v);
            seeds.add(build(obj, sf, p[0], se, sp, key));
            candidates.add(build(obj, cf, p[1], ce, cp, key));
        }
        insert seeds;
        Map<Id,String> labels = new Map<Id,String>();
        for (Integer i = 0; i < seeds.size(); i++) labels.put(seeds[i].Id, String.valueOf(i));
        List<Datacloud.FindDuplicatesResult> found = Datacloud.FindDuplicates.findDuplicates(candidates);
        System.assertEquals(all.size(), found.size(), 'incomplete Find response');
        Database.SaveResult[] saved = Database.insert(candidates, false);
        List<String> outcomes = new List<String>();
        Integer savedCount = 0;
        for (Integer i = 0; i < all.size(); i++) {
            List<Datacloud.DuplicateResult> rules = found[i].getDuplicateResults();
            System.assertEquals(1, rules.size(), 'one default rule expected');
            System.assertEquals(ruleName, rules[0].getDuplicateRule());
            String f = hits(rules[0], labels);
            String d = 'S';
            if (saved[i].isSuccess()) savedCount++;
            else {
                Database.Error[] errs = saved[i].getErrors();
                System.assertEquals(1, errs.size());
                System.assertEquals('DUPLICATES_DETECTED', String.valueOf(errs[0].getStatusCode()));
                Datacloud.DuplicateResult dr = ((Database.DuplicateError) errs[0]).getDuplicateResult();
                System.assertEquals(ruleName, dr.getDuplicateRule());
                d = hits(dr, labels) + (dr.isAllowSave() ? '' : '!');
            }
            outcomes.add(String.valueOf(i) + ':' + f + '/' + d);
        }
        System.assert(outcomes[outcomes.size() - 2].contains('='), 'positive control absent');
        System.assert(outcomes[outcomes.size() - 1].endsWith(':-/S'), 'negative control matched');
        return ruleName + '|saved=' + String.valueOf(savedCount) + '|' + String.join(outcomes, ',');
    }
    public static String m_B2_fwd() {
        return run('Contact', 1000, new List<String>{
            'Ktt/Kttw/std',
            'Pzl/Pzlh/std',
            'Nxc/Nxcg/std',
            'Txr/Txrh/std',
            'Mff/Mffw/std',
            'Dnf/Dnff/std',
            'Bznm/Bznmh/std',
            'Zlfj/Zlfjh/std',
            'Mwgs/Mwgss/std',
            'Zcpg/Zcpgg/std',
            'Zwdx/Zwdxx/std',
            'Xlvf/Xlvff/std'
        });
    }
    public static String m_LeadB2_fwd() {
        return run('Lead', 1050, new List<String>{
            'Ktt/Kttw/std',
            'Pzl/Pzlh/std',
            'Nxc/Nxcg/std',
            'Txr/Txrh/std',
            'Mff/Mffw/std',
            'Dnf/Dnff/std',
            'Bznm/Bznmh/std',
            'Zlfj/Zlfjh/std',
            'Mwgs/Mwgss/std',
            'Zcpg/Zcpgg/std',
            'Zwdx/Zwdxx/std',
            'Xlvf/Xlvff/std'
        });
    }
    public static String m_B2_rev() {
        return run('Contact', 1100, new List<String>{
            'Kttw/Ktt/std',
            'Pzlh/Pzl/std',
            'Nxcg/Nxc/std',
            'Txrh/Txr/std',
            'Mffw/Mff/std',
            'Dnff/Dnf/std',
            'Bznmh/Bznm/std',
            'Zlfjh/Zlfj/std',
            'Mwgss/Mwgs/std',
            'Zcpgg/Zcpg/std',
            'Zwdxx/Zwdx/std',
            'Xlvff/Xlvf/std'
        });
    }
    public static String m_LeadB2_rev() {
        return run('Lead', 1150, new List<String>{
            'Kttw/Ktt/std',
            'Pzlh/Pzl/std',
            'Nxcg/Nxc/std',
            'Txrh/Txr/std',
            'Mffw/Mff/std',
            'Dnff/Dnf/std',
            'Bznmh/Bznm/std',
            'Zlfjh/Zlfj/std',
            'Mwgss/Mwgs/std',
            'Zcpgg/Zcpg/std',
            'Zwdxx/Zwdx/std',
            'Xlvff/Xlvf/std'
        });
    }
}
