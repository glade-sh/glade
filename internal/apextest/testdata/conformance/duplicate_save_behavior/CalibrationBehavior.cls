public class CalibrationBehavior {
    public static String initials() {
        return panel(true, new List<String>{
            'K/Karl','K/Keith','K/Karl','K/Marina'
        });
    }
    public static String transpositions() {
        return panel(false, new List<String>{
            'Ns/Sn',
            'Rens/Resn',
            'Warons/Warosn',
            'Toremins/Toremisn',
            'Li/Il',
            'Reli/Reil',
            'Waroli/Waroil',
            'Toremili/Toremiil',
            'Ht/Th',
            'Reht/Reth',
            'Waroht/Waroth',
            'Toremiht/Toremith',
            'Se/Es',
            'Rese/Rees',
            'Warose/Waroes',
            'Toremise/Toremies',
            'Sz/Zs',
            'Resz/Rezs',
            'Warosz/Warozs',
            'Toremisz/Toremizs',
            'Ty/Yt',
            'Rety/Reyt',
            'Waroty/Waroyt',
            'Toremity/Toremiyt',
            'Sn/Ns',
            'Resn/Rens',
            'Warosn/Warons',
            'Toremisn/Toremins',
            'Il/Li',
            'Reil/Reli',
            'Waroil/Waroli',
            'Toremiil/Toremili',
            'Ab/Ba',
            'Reab/Reba',
            'Waroab/Waroba',
            'Toremiab/Toremiba',
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
    private static String panel(Boolean first, List<String> pairs) {
        List<Contact> seeds = new List<Contact>();
        List<Contact> candidates = new List<Contact>();
        for (Integer i = 0; i < pairs.size(); i++) {
            List<String> p = pairs[i].split('/');
            String mode = p.size() == 3 ? p[2] : 'email';
            String email = 'key' + i + '@pin' + i + '.example';
            Contact seed = new Contact(FirstName = first ? p[0] : 'Alderman', LastName = first ? 'Hartwell' : p[0], Email = email);
            Contact candidate = new Contact(FirstName = first ? p[1] : 'Alderman', LastName = first ? 'Hartwell' : p[1], Email = email);
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
