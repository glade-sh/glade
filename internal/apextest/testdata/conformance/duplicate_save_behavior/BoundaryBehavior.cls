public class BoundaryBehavior {
    public static String sumA() {
        return panel(700, new List<String>{
            'Sobidm/Sobimd',
            'Sobimd/Sobidm',
            'Coriqg/Corigq',
            'Corigq/Coriqg',
            'Haliqt/Halitq',
            'Halitq/Haliqt',
            'Dilocm/Dilomc',
            'Dilomc/Dilocm',
            'Pobiag/Pobiga',
            'Pobiga/Pobiag',
            'Morays/Morasy',
            'Morasy/Morays',
            'Nimasn/Nimans',
            'Nimans/Nimasn',
            'Folatl/Folalt',
            'Folalt/Folatl',
            'Nelark/Nelakr',
            'Nelakr/Nelark',
            'Pelari/Pelair',
            'Pelair/Pelari',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String sumB() {
        return panel(722, new List<String>{
            'Raluhx/Raluxh',
            'Raluxh/Raluhx',
            'Cevigl/Cevilg',
            'Cevilg/Cevigl',
            'Zulosn/Zulons',
            'Zulons/Zulosn',
            'Wulaqt/Wulatq',
            'Wulatq/Wulaqt',
            'Corark/Corakr',
            'Corakr/Corark',
            'Molash/Molahs',
            'Molahs/Molash',
            'Zelifk/Zelikf',
            'Zelikf/Zelifk',
            'Lavoud/Lavodu',
            'Lavodu/Lavoud',
            'Salotl/Salolt',
            'Salolt/Salotl',
            'Varodm/Varomd',
            'Varomd/Varodm',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String sumC() {
        return panel(744, new List<String>{
            'Baduqg/Badugq',
            'Badugq/Baduqg',
            'Fanato/Fanaot',
            'Fanaot/Fanato',
            'Kelalb/Kelabl',
            'Kelabl/Kelalb',
            'Perozb/Perobz',
            'Perobz/Perozb',
            'Voladm/Volamd',
            'Volamd/Voladm',
            'Zilark/Zilakr',
            'Zilakr/Zilark',
            'Favoqt/Favotq',
            'Favotq/Favoqt',
            'Zeraeu/Zeraue',
            'Zeraue/Zeraeu',
            'Gatozb/Gatobz',
            'Gatobz/Gatozb',
            'Femacm/Femamc',
            'Femamc/Femacm',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String sumD() {
        return panel(766, new List<String>{
            'Rd/Dr',
            'Dr/Rd',
            'Tf/Ft',
            'Ft/Tf',
            'Yg/Gy',
            'Gy/Yg',
            'Uh/Hu',
            'Hu/Uh',
            'Dx/Xd',
            'Xd/Dx',
            'Fc/Cf',
            'Cf/Fc',
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
