public class SrtProbe {
    public static List<String> ordered(String dir) {
        List<Srt__c> rows;
        if (dir == 'ASC') {
            rows = [SELECT Name FROM Srt__c ORDER BY Name ASC];
        } else {
            rows = [SELECT Name FROM Srt__c ORDER BY Name DESC];
        }
        List<String> out = new List<String>();
        for (Srt__c r : rows) {
            out.add(r.Name);
        }
        return out;
    }
    public static String slice(String dir, Integer lo, Integer hi) {
        List<String> rows = ordered(dir);
        Map<String, Integer> idx = index();
        List<String> pos = new List<String>();
        for (Integer i = lo; i < hi; i++) {
            pos.add(String.valueOf(idx.get(rows[i])));
        }
        return String.join(pos, ',');
    }
    public static Map<String, Integer> index() {
        Map<String, Integer> m = new Map<String, Integer>();
        List<String> names = SrtSeed.names();
        for (Integer i = 0; i < names.size(); i++) {
            m.put(names[i], i);
        }
        return m;
    }
    public static void seed() {
        delete [SELECT Id FROM Srt__c];
        List<Srt__c> rows = new List<Srt__c>();
        List<String> names = SrtSeed.names();
        for (Integer k = 0; k < 600; k++) {
            Integer i = Math.mod(k * 7 + 13, 600);
            rows.add(new Srt__c(Name = names[i], Pos__c = i));
        }
        insert rows;
    }
}
