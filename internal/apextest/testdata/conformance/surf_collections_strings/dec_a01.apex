List<String> out = new List<String>();
List<String> lits = new List<String>{ '1.5', '0', '-0.01', '123456.789', '1E-8' };
List<String> flds = new List<String>{ 'Curr__c', 'Pct__c', 'Num0__c', 'Num5__c', 'Num8__c' };
for (String lit : lits) {
    Decimal v;
    try {
        v = Decimal.valueOf(lit);
    } catch (Exception e) {
        out.add('A01\tDecimal\t' + lit + '\tvalueOf_error\t\t\t\t\t\t\t\t'
            + String.valueOf(e.getTypeName()) + ': ' + String.valueOf(e.getMessage()).replace('\n', ' '));
        continue;
    }
    for (String f : flds) {
        Dec_Probe__c r = new Dec_Probe__c(Label__c = 'anon ' + lit + ' ' + f);
        if (f == 'Curr__c') { r.Curr__c = v; }
        else if (f == 'Pct__c') { r.Pct__c = v; }
        else if (f == 'Num0__c') { r.Num0__c = v; }
        else if (f == 'Num5__c') { r.Num5__c = v; }
        else { r.Num8__c = v; }
        String dml = 'OK';
        try {
            insert r;
        } catch (DmlException e) {
            dml = 'FAIL ' + String.valueOf(e.getDmlType(0)) + ': ' + String.valueOf(e.getDmlMessage(0)).replace('\n', ' ');
            r = null;
        }
        out.add('A01\t' + f + '\t' + lit + '\tdml\t\t\t\t\t\t\t\t' + dml);
        if (r != null) {
            Dec_Probe__c rb = [SELECT Curr__c, Pct__c, Num0__c, Num5__c, Num8__c
                               FROM Dec_Probe__c WHERE Id = :r.Id];
            Decimal got;
            if (f == 'Curr__c') { got = rb.Curr__c; }
            else if (f == 'Pct__c') { got = rb.Pct__c; }
            else if (f == 'Num0__c') { got = rb.Num0__c; }
            else if (f == 'Num5__c') { got = rb.Num5__c; }
            else { got = rb.Num8__c; }
            out.add('A01\t' + f + '\t' + lit + '\treadback\t' + String.valueOf(got) + '\t'
                + got.precision() + '\t' + got.scale() + '\t' + String.valueOf(got.hashCode()) + '\t'
                + String.valueOf((new List<Decimal>{ got }).hashCode()) + '\t\t\tsoql');
        }
    }
}
out.add('END');
System.debug(String.join(out, '
'));
