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
    public static String m_H_noemail_fwd() {
        return run('Contact', 1000, new List<String>{
            'Ikir/Ikirp/noemail',
            'Sznc/Szncs/noemail',
            'Cpv/Cpvp/noemail',
            'Pszkm/Pszkn/noemail',
            'Bxc/Bcx/noemail',
            'Lysqw/Lyqws/noemail',
            'Gklnjdrl/Gklnjdlr/noemail',
            'Riim/Riimq/noemail'
        });
    }
    public static String m_H_noemail_rev() {
        return run('Contact', 1050, new List<String>{
            'Ikirp/Ikir/noemail',
            'Szncs/Sznc/noemail',
            'Cpvp/Cpv/noemail',
            'Pszkn/Pszkm/noemail',
            'Bcx/Bxc/noemail',
            'Lyqws/Lysqw/noemail',
            'Gklnjdlr/Gklnjdrl/noemail',
            'Riimq/Riim/noemail'
        });
    }
    public static String m_H_diffemail_fwd() {
        return run('Contact', 1100, new List<String>{
            'Ikir/Ikirp/diffemail',
            'Sznc/Szncs/diffemail',
            'Cpv/Cpvp/diffemail',
            'Pszkm/Pszkn/diffemail',
            'Bxc/Bcx/diffemail',
            'Lysqw/Lyqws/diffemail',
            'Gklnjdrl/Gklnjdlr/diffemail',
            'Riim/Riimq/diffemail'
        });
    }
    public static String m_H_diffemail_rev() {
        return run('Contact', 1150, new List<String>{
            'Ikirp/Ikir/diffemail',
            'Szncs/Sznc/diffemail',
            'Cpvp/Cpv/diffemail',
            'Pszkn/Pszkm/diffemail',
            'Bcx/Bxc/diffemail',
            'Lyqws/Lysqw/diffemail',
            'Gklnjdlr/Gklnjdrl/diffemail',
            'Riimq/Riim/diffemail'
        });
    }
    public static String m_H_diffname_fwd() {
        return run('Contact', 1200, new List<String>{
            'Ikir/Ikirp/diffname',
            'Sznc/Szncs/diffname',
            'Cpv/Cpvp/diffname',
            'Pszkm/Pszkn/diffname',
            'Bxc/Bcx/diffname',
            'Lysqw/Lyqws/diffname',
            'Gklnjdrl/Gklnjdlr/diffname',
            'Riim/Riimq/diffname'
        });
    }
    public static String m_H_diffname_rev() {
        return run('Contact', 1250, new List<String>{
            'Ikirp/Ikir/diffname',
            'Szncs/Sznc/diffname',
            'Cpvp/Cpv/diffname',
            'Pszkn/Pszkm/diffname',
            'Bcx/Bxc/diffname',
            'Lyqws/Lysqw/diffname',
            'Gklnjdlr/Gklnjdrl/diffname',
            'Riimq/Riim/diffname'
        });
    }
    public static String m_H_phone_fwd() {
        return run('Contact', 1300, new List<String>{
            'Ikir/Ikirp/phone',
            'Sznc/Szncs/phone',
            'Cpv/Cpvp/phone',
            'Pszkm/Pszkn/phone',
            'Bxc/Bcx/phone',
            'Lysqw/Lyqws/phone',
            'Gklnjdrl/Gklnjdlr/phone',
            'Riim/Riimq/phone'
        });
    }
    public static String m_H_phone_rev() {
        return run('Contact', 1350, new List<String>{
            'Ikirp/Ikir/phone',
            'Szncs/Sznc/phone',
            'Cpvp/Cpv/phone',
            'Pszkn/Pszkm/phone',
            'Bcx/Bxc/phone',
            'Lyqws/Lysqw/phone',
            'Gklnjdlr/Gklnjdrl/phone',
            'Riimq/Riim/phone'
        });
    }
    public static String m_I_upper_fwd() {
        return run('Contact', 1400, new List<String>{
            'IKIR/IKIRP/std',
            'SZNC/SZNCS/std',
            'PSZKM/PSZKN/std',
            'FLRYM/FLRYN/std'
        });
    }
    public static String m_I_upper_rev() {
        return run('Contact', 1450, new List<String>{
            'IKIRP/IKIR/std',
            'SZNCS/SZNC/std',
            'PSZKN/PSZKM/std',
            'FLRYN/FLRYM/std'
        });
    }
    public static String m_I_accent_fwd() {
        return run('Contact', 1500, new List<String>{
            'Iékir/Iékirp/std',
            'Séznc/Sézncs/std',
            'Pészkm/Pészkn/std',
            'Félrym/Félryn/std'
        });
    }
    public static String m_I_accent_rev() {
        return run('Contact', 1550, new List<String>{
            'Iékirp/Iékir/std',
            'Sézncs/Séznc/std',
            'Pészkn/Pészkm/std',
            'Félryn/Félrym/std'
        });
    }
    public static String m_I_hyphen_fwd() {
        return run('Contact', 1600, new List<String>{
            'Ik-ir/Ik-irp/std',
            'Sz-nc/Sz-ncs/std',
            'Ps-zkm/Ps-zkn/std',
            'Fl-rym/Fl-ryn/std'
        });
    }
    public static String m_I_hyphen_rev() {
        return run('Contact', 1650, new List<String>{
            'Ik-irp/Ik-ir/std',
            'Sz-ncs/Sz-nc/std',
            'Ps-zkn/Ps-zkm/std',
            'Fl-ryn/Fl-rym/std'
        });
    }
    public static String m_I_token_fwd() {
        return run('Contact', 1700, new List<String>{
            'Van Ikir/Van Ikirp/std',
            'Van Sznc/Van Szncs/std',
            'Van Pszkm/Van Pszkn/std',
            'Van Flrym/Van Flryn/std'
        });
    }
    public static String m_I_token_rev() {
        return run('Contact', 1750, new List<String>{
            'Van Ikirp/Van Ikir/std',
            'Van Szncs/Van Sznc/std',
            'Van Pszkn/Van Pszkm/std',
            'Van Flryn/Van Flrym/std'
        });
    }
}
