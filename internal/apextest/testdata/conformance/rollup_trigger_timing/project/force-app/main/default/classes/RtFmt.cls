public class RtFmt {
    public static String of(RtParent__c p) {
        return 'c=' + dec(p.Count__c) + ',s=' + dec(p.SumAmt__c) + ',mn=' + dec(p.MinAmt__c)
            + ',mx=' + dec(p.MaxAmt__c) + ',fs=' + dec(p.FiltSum__c) + ',fc=' + dec(p.FiltCount__c);
    }
    public static String dec(Decimal d) {
        return d == null ? '' : String.valueOf(d);
    }
}
