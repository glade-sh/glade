public class GradeBehavior {
    public static String gradeA() {
        return panel(1100, new List<String>{
            'Ygh/Yhg',
            'Ccv/Cvc',
            'Hrt/Htr',
            'Ddf/Dfd',
            'Duvolitp/Duvolipt',
            'Hirenodk/Hirenokd',
            'Kelunofl/Kelunolf',
            'Muyaviwu/Muyaviuw',
            'Awbfg/Awfgb',
            'Boedc/Boced',
            'Pavfr/Pafrv',
            'Fepol/Felpo',
            'Jlun/Jlunf',
            'Rynk/Rynkg',
            'Tulm/Tulmk',
            'Yent/Yentp',
            'Jqp/Jpq',
            'Wunipapq/Wunipaqp',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String gradeB() {
        return panel(1120, new List<String>{
            'Yhg/Ygh',
            'Cvc/Ccv',
            'Htr/Hrt',
            'Dfd/Ddf',
            'Duvolipt/Duvolitp',
            'Hirenokd/Hirenodk',
            'Kelunolf/Kelunofl',
            'Muyaviuw/Muyaviwu',
            'Awfgb/Awbfg',
            'Boced/Boedc',
            'Pafrv/Pavfr',
            'Felpo/Fepol',
            'Jlunf/Jlun',
            'Rynkg/Rynk',
            'Tulmk/Tulm',
            'Yentp/Yent',
            'Jpq/Jqp',
            'Wunipaqp/Wunipapq',
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
