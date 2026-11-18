public class LocalEditsBehavior {
    public static String editsA() {
        return panel(900, new List<String>{
            'Rume/Rumed',
            'Cavne/Cavn',
            'Fult/Fultz',
            'Zelas/Zela',
            'Mutgh/Mught',
            'Muhtg/Mutgh',
            'Zirty/Ziyrt',
            'Lokoj/Lojko',
            'Aht/Ath',
            'Zds/Zsd',
            'Qaer/Qare',
            'Vogf/Vofg',
            'Lirty/Liryt',
            'Bemvc/Bemcv',
            'Porelsd/Porelds',
            'Zetanht/Zetanth',
            'Gafunibn/Gafuninb',
            'Veromixa/Veromiax',
            'Lumers/Lumres',
            'Bofrae/Bofare',
            'Kusd/Ksdu',
            'Zamrte/Zamert',
            'Nel/Nelz',
            'Rumed/Rume',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String editsB() {
        return panel(926, new List<String>{
            'Kesh/Kesha',
            'Fultz/Fult',
            'Pirm/Pirmm',
            'Mught/Mutgh',
            'Zirty/Zityr',
            'Ziyrt/Zirty',
            'Lojko/Loojk',
            'Ath/Aht',
            'Uqp/Upq',
            'Qare/Qaer',
            'Duqp/Dupq',
            'Liryt/Lirty',
            'Falzm/Falmz',
            'Porelds/Porelsd',
            'Bovilqk/Bovilkq',
            'Gafuninb/Gafunibn',
            'Cudelarl/Cudelalr',
            'Lumres/Lumers',
            'Gore/Goyre',
            'Ksdu/Kusd',
            'Sorin/Sorim',
            'Nelz/Nel',
            'Cavn/Cavne',
            'Kesha/Kesh',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String editsC() {
        return panel(952, new List<String>{
            'Zela/Zelas',
            'Pirmm/Pirm',
            'Mutgh/Muhtg',
            'Zityr/Zirty',
            'Lojko/Lokoj',
            'Loojk/Lojko',
            'Zsd/Zds',
            'Upq/Uqp',
            'Vofg/Vogf',
            'Dupq/Duqp',
            'Bemcv/Bemvc',
            'Falmz/Falzm',
            'Zetanth/Zetanht',
            'Bovilkq/Bovilqk',
            'Veromiax/Veromixa',
            'Cudelalr/Cudelarl',
            'Bofare/Bofrae',
            'Goyre/Gore',
            'Zamert/Zamrte',
            'Sorim/Sorin',
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
