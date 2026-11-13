public class PrefixBehavior {
    public static String stemA() {
        return panel(200, new List<String>{
            'Nebusn/Nebuns',
            'Nebuns/Nebusn',
            'Neburk/Nebukr',
            'Nebukr/Neburk',
            'Nebisn/Nebins',
            'Nebins/Nebisn',
            'Nebirk/Nebikr',
            'Nebikr/Nebirk',
            'Nebasn/Nebans',
            'Nebans/Nebasn',
            'Nebark/Nebakr',
            'Nebakr/Nebark',
            'Nebosn/Nebons',
            'Nebons/Nebosn',
            'Nebork/Nebokr',
            'Nebokr/Nebork',
            'Nebesn/Nebens',
            'Nebens/Nebesn',
            'Neberk/Nebekr',
            'Nebekr/Neberk',
            'Tavisn/Tavins',
            'Tavins/Tavisn',
            'Tavirk/Tavikr',
            'Tavikr/Tavirk',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String stemB() {
        return panel(226, new List<String>{
            'Tavusn/Tavuns',
            'Tavuns/Tavusn',
            'Tavurk/Tavukr',
            'Tavukr/Tavurk',
            'Tavasn/Tavans',
            'Tavans/Tavasn',
            'Tavark/Tavakr',
            'Tavakr/Tavark',
            'Tavosn/Tavons',
            'Tavons/Tavosn',
            'Tavork/Tavokr',
            'Tavokr/Tavork',
            'Tavesn/Tavens',
            'Tavens/Tavesn',
            'Taverk/Tavekr',
            'Tavekr/Taverk',
            'Tebusn/Tebuns',
            'Tebuns/Tebusn',
            'Teburk/Tebukr',
            'Tebukr/Teburk',
            'Navisn/Navins',
            'Navins/Navisn',
            'Navirk/Navikr',
            'Navikr/Navirk',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String stemC() {
        return panel(252, new List<String>{
            'Warosn/Warons',
            'Warons/Warosn',
            'Warork/Warokr',
            'Warokr/Warork',
            'Warisn/Warins',
            'Warins/Warisn',
            'Warirk/Warikr',
            'Warikr/Warirk',
            'Warusn/Waruns',
            'Waruns/Warusn',
            'Warurk/Warukr',
            'Warukr/Warurk',
            'Navesn/Navens',
            'Navens/Navesn',
            'Naverk/Navekr',
            'Navekr/Naverk',
            'Havosn/Havons',
            'Havons/Havosn',
            'Havork/Havokr',
            'Havokr/Havork',
            'Nebuqg/Nebugq',
            'Nebugq/Nebuqg',
            'Nebiqg/Nebigq',
            'Nebigq/Nebiqg',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String stemD() {
        return panel(278, new List<String>{
            'Nebaqg/Nebagq',
            'Nebagq/Nebaqg',
            'Neboqg/Nebogq',
            'Nebogq/Neboqg',
            'Nebeqg/Nebegq',
            'Nebegq/Nebeqg',
            'Taviqg/Tavigq',
            'Tavigq/Taviqg',
            'Tavuqg/Tavugq',
            'Tavugq/Tavuqg',
            'Tavaqg/Tavagq',
            'Tavagq/Tavaqg',
            'Tavoqg/Tavogq',
            'Tavogq/Tavoqg',
            'Taveqg/Tavegq',
            'Tavegq/Taveqg',
            'Tebuqg/Tebugq',
            'Tebugq/Tebuqg',
            'Naviqg/Navigq',
            'Navigq/Naviqg',
            'Waroqg/Warogq',
            'Warogq/Waroqg',
            'Wariqg/Warigq',
            'Warigq/Wariqg',
            'Kale/Kale',
            'Li/Polk'
        });
    }
    public static String stemE() {
        return panel(304, new List<String>{
            'Nebuqt/Nebutq',
            'Nebutq/Nebuqt',
            'Nebiqt/Nebitq',
            'Nebitq/Nebiqt',
            'Taviqt/Tavitq',
            'Tavitq/Taviqt',
            'Tavuqt/Tavutq',
            'Tavutq/Tavuqt',
            'Tebuqt/Tebutq',
            'Tebutq/Tebuqt',
            'Naviqt/Navitq',
            'Navitq/Naviqt',
            'Waroqt/Warotq',
            'Warotq/Waroqt',
            'Wariqt/Waritq',
            'Waritq/Wariqt',
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
