public class KeyboardBehavior {
    public static String shortA() {
        return panel(0, new List<String>{
            'Qw/Wq',
            'Wq/Qw',
            'We/Ew',
            'Ew/We',
            'Er/Re',
            'Re/Er',
            'Rt/Tr',
            'Tr/Rt',
            'Ty/Yt',
            'Yt/Ty',
            'Yu/Uy',
            'Uy/Yu',
            'Ui/Iu',
            'Iu/Ui',
            'Io/Oi',
            'Oi/Io',
            'Op/Po',
            'Po/Op',
            'As/Sa',
            'Sa/As',
            'Sd/Ds',
            'Ds/Sd',
            'Df/Fd',
            'Fd/Df',
            'Fg/Gf',
            'Gf/Fg',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String shortB() {
        return panel(28, new List<String>{
            'Gh/Hg',
            'Hg/Gh',
            'Hj/Jh',
            'Jh/Hj',
            'Jk/Kj',
            'Kj/Jk',
            'Kl/Lk',
            'Lk/Kl',
            'Zx/Xz',
            'Xz/Zx',
            'Xc/Cx',
            'Cx/Xc',
            'Cv/Vc',
            'Vc/Cv',
            'Vb/Bv',
            'Bv/Vb',
            'Bn/Nb',
            'Nb/Bn',
            'Nm/Mn',
            'Mn/Nm',
            'Qs/Sq',
            'Sq/Qs',
            'Wd/Dw',
            'Dw/Wd',
            'Ef/Fe',
            'Fe/Ef',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String shortC() {
        return panel(56, new List<String>{
            'Rg/Gr',
            'Gr/Rg',
            'Th/Ht',
            'Ht/Th',
            'Yj/Jy',
            'Jy/Yj',
            'Uk/Ku',
            'Ku/Uk',
            'Il/Li',
            'Li/Il',
            'Ax/Xa',
            'Xa/Ax',
            'Sc/Cs',
            'Cs/Sc',
            'Dv/Vd',
            'Vd/Dv',
            'Fb/Bf',
            'Bf/Fb',
            'Gn/Ng',
            'Ng/Gn',
            'Hm/Mh',
            'Mh/Hm',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String longA() {
        return panel(80, new List<String>{
            'Nebuqg/Nebugq',
            'Nebugq/Nebuqg',
            'Nebuwh/Nebuhw',
            'Nebuhw/Nebuwh',
            'Nebuej/Nebuje',
            'Nebuje/Nebuej',
            'Neburk/Nebukr',
            'Nebukr/Neburk',
            'Nebutl/Nebult',
            'Nebult/Nebutl',
            'Nebuab/Nebuba',
            'Nebuba/Nebuab',
            'Nebusn/Nebuns',
            'Nebuns/Nebusn',
            'Nebudm/Nebumd',
            'Nebumd/Nebudm',
            'Taviqg/Tavigq',
            'Tavigq/Taviqg',
            'Taviwh/Tavihw',
            'Tavihw/Taviwh',
            'Taviej/Tavije',
            'Tavije/Taviej',
            'Tavirk/Tavikr',
            'Tavikr/Tavirk',
            'Tavitl/Tavilt',
            'Tavilt/Tavitl',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String longB() {
        return panel(108, new List<String>{
            'Taviab/Taviba',
            'Taviba/Taviab',
            'Tavisn/Tavins',
            'Tavins/Tavisn',
            'Tavidm/Tavimd',
            'Tavimd/Tavidm',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String nearA() {
        return panel(116, new List<String>{
            'Qa/Aq',
            'Aq/Qa',
            'Wa/Aw',
            'Aw/Wa',
            'Qe/Eq',
            'Eq/Qe',
            'Qd/Dq',
            'Dq/Qd',
            'Qx/Xq',
            'Xq/Qx',
            'Nebuqr/Neburq',
            'Neburq/Nebuqr',
            'Nebuqt/Nebutq',
            'Nebutq/Nebuqt',
            'Nebuqf/Nebufq',
            'Nebufq/Nebuqf',
            'Nebuqh/Nebuhq',
            'Nebuhq/Nebuqh',
            'Nebuta/Nebuat',
            'Nebuat/Nebuta',
            'Nebuqb/Nebubq',
            'Nebubq/Nebuqb',
            'Rer/Rre',
            'Rre/Rer',
            'Rax/Rxa',
            'Rxa/Rax',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String nearB() {
        return panel(144, new List<String>{
            'Uleer/Ulere',
            'Ulere/Uleer',
            'Uleax/Ulexa',
            'Ulexa/Uleax',
            'Tavier/Tavire',
            'Tavire/Tavier',
            'Taviax/Tavixa',
            'Tavixa/Taviax',
            'Nubwh/Nubhw',
            'Nubhw/Nubwh',
            'Nubsn/Nubns',
            'Nubns/Nubsn',
            'Raviewh/Raviehw',
            'Raviehw/Raviewh',
            'Raviesn/Raviens',
            'Raviens/Raviesn',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    private static String hits(Datacloud.DuplicateResult rule, Map<Id,String> labels) {
        List<String> found = new List<String>();
        for (Datacloud.MatchResult result : rule.getMatchResults()) {
            for (Datacloud.MatchRecord match : result.getMatchRecords()) {
                Id matchedId = match.getRecord().Id;
                System.assert(labels.containsKey(matchedId), 'unidentified seed');
                found.add(labels.get(matchedId) + '=' + String.valueOf(match.getMatchConfidence().intValue()));
            }
        }
        found.sort();
        return found.isEmpty() ? '-' : String.join(found, '+');
    }
    private static String panel(Integer startKey, List<String> pairs) {
        List<Contact> seeds = new List<Contact>();
        List<Contact> candidates = new List<Contact>();
        for (Integer i = 0; i < pairs.size(); i++) {
            List<String> p = pairs[i].split('/');
            String key = String.valueOf(startKey + i);
            String email = 's' + key + '@k' + key + '.example';
            Contact seed = new Contact(FirstName = 'Alderman', LastName = p[0], Email = email);
            Contact candidate = new Contact(FirstName = 'Alderman', LastName = p[1], Email = email);
            seeds.add(seed);
            candidates.add(candidate);
        }
        insert seeds;
        Map<Id,String> labels = new Map<Id,String>();
        for (Integer i = 0; i < seeds.size(); i++) labels.put(seeds[i].Id, String.valueOf(i));
        List<Datacloud.FindDuplicatesResult> found = Datacloud.FindDuplicates.findDuplicates(candidates);
        System.assertEquals(pairs.size(), found.size(), 'incomplete Find response');
        Database.SaveResult[] saved = Database.insert(candidates, false);
        List<String> outcomes = new List<String>();
        for (Integer i = 0; i < pairs.size(); i++) {
            List<Datacloud.DuplicateResult> rules = found[i].getDuplicateResults();
            System.assertEquals(1, rules.size(), 'default Contact rule expected');
            System.assertEquals('Standard_Contact_Duplicate_Rule', rules[0].getDuplicateRule());
            String f = hits(rules[0], labels);
            String d = 'S';
            if (!saved[i].isSuccess()) {
                Database.Error[] errs = saved[i].getErrors();
                System.assertEquals(1, errs.size());
                System.assertEquals('DUPLICATES_DETECTED', String.valueOf(errs[0].getStatusCode()));
                Datacloud.DuplicateResult dr = ((Database.DuplicateError)errs[0]).getDuplicateResult();
                System.assertEquals('Standard_Contact_Duplicate_Rule', dr.getDuplicateRule());
                System.assert(dr.isAllowSave());
                d = hits(dr, labels);
            }
            outcomes.add(String.valueOf(i) + ':' + f + '/' + d);
        }
        System.assert(outcomes[outcomes.size()-2].contains('='), 'positive control absent');
        System.assert(outcomes[outcomes.size()-1].endsWith(':-/S'), 'negative control matched');
        return String.join(outcomes, ',');
    }
}
