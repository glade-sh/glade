public class OrdSeed {
    public static Ord__c row(Integer r) {
        String nm = (r <= 6 ? 'K' : 'X') + (r < 10 ? '0' : '') + String.valueOf(r);
        String grp;
        Integer m = Math.mod(r, 4);
        if (m == 1) {
            grp = 'Alpha';
        } else if (m == 2) {
            grp = 'Bravo';
        } else if (m == 3) {
            grp = 'Charlie';
        } else {
            grp = 'Delta';
        }
        return new Ord__c(Name = nm, Grp__c = grp, Amt__c = r * 10, Rnk__c = r);
    }
}
