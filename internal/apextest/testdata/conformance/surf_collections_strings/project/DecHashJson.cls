@IsTest
private class DecHashJson {
    static String row(String id, String kind, String lit, String phase, String s, String notes) {
        List<String> cols = new List<String>{ id, kind, lit, phase, s, '', '', '', '', '', '', notes };
        return String.join(cols, '\t');
    }

    static void emitFail(List<String> lines) {
        lines.add('END');
        System.assert(false, '\n' + String.join(lines, '\n'));
    }

    @IsTest static void S10() {
        List<String> out = new List<String>();
        Dec_Probe__c r = new Dec_Probe__c(Label__c = 'S10', Curr__c = 1.5, Pct__c = 1.5,
            Num0__c = 1, Num5__c = 1.5, Num8__c = 1.5);
        insert r;
        Dec_Probe__c rb = [SELECT Curr__c, Pct__c, Num0__c, Num5__c, Num8__c
                           FROM Dec_Probe__c WHERE Id = :r.Id];
        out.add(row('S10', 'Decimal', '1.500', 'json', JSON.serialize(Decimal.valueOf('1.500')),
            'serialize_literal'));
        out.add(row('S10', 'Decimal', '1.5', 'json', JSON.serialize(Decimal.valueOf('1.5')),
            'serialize_literal'));
        out.add(row('S10', 'Curr__c', '1.5', 'json_readback', JSON.serialize(rb.Curr__c), 'serialize_readback'));
        out.add(row('S10', 'Pct__c', '1.5', 'json_readback', JSON.serialize(rb.Pct__c), 'serialize_readback'));
        out.add(row('S10', 'Num0__c', '1', 'json_readback', JSON.serialize(rb.Num0__c), 'serialize_readback'));
        out.add(row('S10', 'Num5__c', '1.5', 'json_readback', JSON.serialize(rb.Num5__c), 'serialize_readback'));
        out.add(row('S10', 'Num8__c', '1.5', 'json_readback', JSON.serialize(rb.Num8__c), 'serialize_readback'));
        emitFail(out);
    }
}
