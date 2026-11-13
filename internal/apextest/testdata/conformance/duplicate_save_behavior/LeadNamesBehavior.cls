public class LeadNamesBehavior {
    public static String initials() {
        return panel(true, new List<String>{
            'J/Jonathan',
            'Jonathan/J',
            'J./Jonathan',
            'Jonathan/J.',
            'J/Marina',
            'Marina/J',
            'J/Jonathan/none',
            'Jonathan/J/none',
            'J/Jonathan/phone',
            'Jonathan/J/phone',
            'J/Jonathan/wrong',
            'Jonathan/J/other',
            'Li/Li',
            'Q/Marina'
        });
    }
    public static String surnames() {
        return panel(false, new List<String>{
            'Smiht/Smith',
            'Smith/Smiht',
            'Havens/Havesn',
            'Havesn/Havens',
            'Nimasn/Nimans',
            'Nimans/Nimasn',
            'Nelark/Nelakr',
            'Nelakr/Nelark',
            'Zulosn/Zulons',
            'Zulons/Zulosn',
            'Corark/Corakr',
            'Corakr/Corark',
            'Smiht/Smith/none',
            'Smith/Smiht/phone',
            'Havens/Havesn/other',
            'Havesn/Havens/wrong',
            'Zulosn/Zulons/none',
            'Zulons/Zulosn/wrong',
            'Zulosn/Zulons/phone',
            'Nelark/Nelakr/phone',
            'Smith/Smith',
            'Li/Polk'
        });
    }
    private static String hits(Datacloud.DuplicateResult rule, Map<Id,String> labels) {
        List<String> found = new List<String>();
        for (Datacloud.MatchResult result : rule.getMatchResults()) {
            System.assertEquals('Lead', result.getEntityType(), 'expected same-object matching');
            System.assertEquals('Standard_Lead_Match_Rule_v1_0', result.getRule(), 'expected standard Lead matching rule');
            for (Datacloud.MatchRecord match : result.getMatchRecords()) {
                Id matchedId = match.getRecord().Id;
                System.assert(labels.containsKey(matchedId), 'unidentified seed');
                found.add(labels.get(matchedId) + '=' + String.valueOf(match.getMatchConfidence().intValue()));
            }
        }
        found.sort();
        return found.isEmpty() ? '-' : String.join(found, '+');
    }
    private static String panel(Boolean first, List<String> pairs) {
        List<Lead> seeds = new List<Lead>();
        List<Lead> candidates = new List<Lead>();
        for (Integer i = 0; i < pairs.size(); i++) {
            List<String> p = pairs[i].split('/');
            String mode = p.size() == 3 ? p[2] : 'email';
            String email = 'key' + i + '@pin' + i + '.example';
            Lead seed = new Lead(Company = 'Beacon', FirstName = first ? p[0] : 'Alderman', LastName = first ? 'Hartwell' : p[0], Email = email);
            Lead candidate = new Lead(Company = 'Oxbow', FirstName = first ? p[1] : 'Alderman', LastName = first ? 'Hartwell' : p[1], Email = email);
            if (mode == 'none' || mode == 'phone') {
                seed.Email = null;
                candidate.Email = null;
            }
            if (mode == 'phone') {
                seed.Phone = '(734) ' + String.valueOf(300 + i) + '-0189';
                candidate.Phone = seed.Phone;
            }
            if (mode == 'wrong') {
                if (first) candidate.LastName = 'Rosencrantz';
                else candidate.FirstName = 'Quentin';
            }
            if (mode == 'other' || mode == 'wrongEmail') candidate.Email = 'other' + i + '@elsewhere.example';
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
            System.assertEquals(1, rules.size(), 'default Lead rule expected');
            System.assertEquals('Standard_Lead_Duplicate_Rule', rules[0].getDuplicateRule());
            String f = (rules[0].isAllowSave() ? 'T' : 'F') + hits(rules[0], labels);
            String d = 'S';
            if (!saved[i].isSuccess()) {
                Database.Error[] errs = saved[i].getErrors();
                System.assertEquals(1, errs.size());
                System.assertEquals('DUPLICATES_DETECTED', String.valueOf(errs[0].getStatusCode()));
                Datacloud.DuplicateResult dr = ((Database.DuplicateError)errs[0]).getDuplicateResult();
                System.assertEquals('Standard_Lead_Duplicate_Rule', dr.getDuplicateRule());
                d = (dr.isAllowSave() ? 'T' : 'F') + hits(dr, labels);
            }
            outcomes.add(String.valueOf(i) + ':' + f + '/' + d);
        }
        System.assert(outcomes[outcomes.size()-2].contains('='), 'positive control absent');
        System.assert(outcomes[outcomes.size()-1].endsWith('-/S'), 'negative control matched');
        return String.join(outcomes, ',');
    }
}
