public class NamePanelBehavior {
    public static String initials() {
        return panel(true, new List<String>{
            'J/Jonathan','Jonathan/J','M/Marina','Marina/M','Q/Quentin','Quentin/Q',
            'A/Alina','Alina/A','Z/Zavian','Zavian/Z','T/Tamsen','Tamsen/T',
            'B/Brina','Brina/B','M/Quentin','T/Brina','J./Jonathan','JR/John',
            'Jo/Jonathan','J/Jonathan/none','Jonathan/J/other','J/Jonathan/wrong',
            'J/Jonathan/phone','Jonathan/J/phone','J/Jo','Jo/J','M/Marina/wrongEmail',
            'M/M','Q/Marina'
        });
    }
    public static String transpositions() {
        return panel(false, new List<String>{
            'Smith/Smiht','Smiht/Smith','Moses/Mosse','Mosse/Moses','Parker/Parkre',
            'Parkre/Parker','Havens/Havesn','Havesn/Havens','Wintor/Wintro','Wintro/Wintor',
            'Parrk/Parkr','Parkr/Parrk','Martinezk/Martinekz','Martinekz/Martinezk',
            'Denvor/Denvro','Denvro/Denvor','Kristal/Kristla','Kristla/Kristal',
            'Tavern/Tavren','Smith/Msith','Smith/Smhit','Smith/Smuth','Smith/Smit',
            'Li/Il','Denvor/Benvro','Smith/Smiht/none','Smith/Smiht/wrong',
            'Smith/Smiht/phone','Smith/Smith','Li/Polk'
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
