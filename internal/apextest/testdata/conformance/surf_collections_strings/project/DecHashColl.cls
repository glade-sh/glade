@IsTest
private class DecHashColl {
    static String row(String id, String kind, String lit, String phase, String s, String h,
                      String size, String con, String notes) {
        List<String> cols = new List<String>{ id, kind, lit, phase, s, '', '', h, '', size, con, notes };
        return String.join(cols, '\t');
    }

    static void emitFail(List<String> lines) {
        lines.add('END');
        System.assert(false, '\n' + String.join(lines, '\n'));
    }

    @IsTest static void S07a() {
        List<String> out = new List<String>();
        Set<Decimal> s = new Set<Decimal>{ 1.5 };
        out.add(row('S07a', 'Set', '1.5', 'set_membership', '', String.valueOf(s.hashCode()),
            String.valueOf(s.size()), String.valueOf(s.contains(1.50)), 'add_1.5_contains_literal_1.50'));
        out.add(row('S07a', 'Set', '1.5', 'set_membership', '', '', String.valueOf(s.size()),
            String.valueOf(s.contains(Decimal.valueOf('1.500'))), 'contains_valueOf_1.500'));
        Set<Decimal> z = new Set<Decimal>{ Decimal.valueOf('0') };
        out.add(row('S07a', 'Set', '0', 'set_membership', '', String.valueOf(z.hashCode()),
            String.valueOf(z.size()), String.valueOf(z.contains(0.00)), 'add_0_contains_literal_0.00'));
        out.add(row('S07a', 'Set', '0', 'set_membership', '', '', String.valueOf(z.size()),
            String.valueOf(z.contains(Decimal.valueOf('0.000'))), 'contains_valueOf_0.000'));
        emitFail(out);
    }

    @IsTest static void S07b() {
        List<String> out = new List<String>();
        Set<Decimal> s = new Set<Decimal>{ Decimal.valueOf('-0.01') };
        out.add(row('S07b', 'Set', '-0.01', 'set_membership', '', String.valueOf(s.hashCode()),
            String.valueOf(s.size()), String.valueOf(s.contains(-0.0100)),
            'add_-0.01_contains_literal_-0.0100'));
        out.add(row('S07b', 'Set', '-0.01', 'set_membership', '', '', String.valueOf(s.size()),
            String.valueOf(s.contains(Decimal.valueOf('-0.010000'))), 'contains_valueOf_-0.010000'));
        emitFail(out);
    }

    @IsTest static void S08a() {
        List<String> out = new List<String>();
        Map<Decimal, String> m = new Map<Decimal, String>();
        m.put(1.5, 'one_five');
        out.add(row('S08a', 'Map', '1.5', 'map_lookup', String.valueOf(m.get(1.50)),
            '', String.valueOf(m.size()), String.valueOf(m.containsKey(1.50)), 'get_literal_1.50'));
        out.add(row('S08a', 'Map', '1.5', 'map_lookup', String.valueOf(m.get(Decimal.valueOf('1.500'))),
            '', String.valueOf(m.size()), String.valueOf(m.containsKey(Decimal.valueOf('1.500'))),
            'get_valueOf_1.500'));
        Map<Decimal, String> z = new Map<Decimal, String>();
        z.put(Decimal.valueOf('0'), 'zero');
        out.add(row('S08a', 'Map', '0', 'map_lookup', String.valueOf(z.get(0.000)),
            '', String.valueOf(z.size()), String.valueOf(z.containsKey(0.00)), 'get_literal_0.000'));
        emitFail(out);
    }

    @IsTest static void S08b() {
        List<String> out = new List<String>();
        Map<Decimal, String> m = new Map<Decimal, String>();
        m.put(Decimal.valueOf('-0.01'), 'neg');
        out.add(row('S08b', 'Map', '-0.01', 'map_lookup', String.valueOf(m.get(-0.0100)),
            '', String.valueOf(m.size()), String.valueOf(m.containsKey(-0.0100)), 'get_literal_-0.0100'));
        out.add(row('S08b', 'Map', '-0.01', 'map_lookup',
            String.valueOf(m.get(Decimal.valueOf('-0.010000'))), '', String.valueOf(m.size()),
            String.valueOf(m.containsKey(Decimal.valueOf('-0.010000'))), 'get_valueOf_-0.010000'));
        emitFail(out);
    }

    @IsTest static void S09() {
        List<String> out = new List<String>();
        List<Decimal> l1 = new List<Decimal>{ 1.5 };
        List<Decimal> l2 = new List<Decimal>{ 1.50 };
        List<Decimal> l3 = new List<Decimal>{ Decimal.valueOf('1.500') };
        out.add(row('S09', 'List', '1.5', 'list_hash', '', String.valueOf(l1.hashCode()), '', '',
            'l1_eq_l2=' + (l1 == l2)));
        out.add(row('S09', 'List', '1.50', 'list_hash', '', String.valueOf(l2.hashCode()), '', '',
            'l2_eq_l3=' + (l2 == l3)));
        out.add(row('S09', 'List', '1.500', 'list_hash', '', String.valueOf(l3.hashCode()), '', '',
            'l3_eq_l1=' + (l3 == l1)));
        emitFail(out);
    }

    @IsTest static void S09b() {
        List<String> out = new List<String>();
        Dec_Probe__c r = new Dec_Probe__c(Label__c = 'S09b', Curr__c = 1.5, Num5__c = 1.5);
        insert r;
        Dec_Probe__c rb = [SELECT Curr__c, Num5__c FROM Dec_Probe__c WHERE Id = :r.Id];
        List<Decimal> mem = new List<Decimal>{ 1.5 };
        List<Decimal> read = new List<Decimal>{ rb.Curr__c };
        out.add(row('S09b', 'List', '1.5', 'list_hash_memory', '', String.valueOf(mem.hashCode()),
            '', '', 'memory_curr'));
        out.add(row('S09b', 'List', '1.5', 'list_hash_readback', '', String.valueOf(read.hashCode()),
            '', '', 'readback_curr'));
        out.add(row('S09b', 'List', '1.5', 'list_hash_readback', '', String.valueOf(
            new List<Decimal>{ rb.Num5__c }.hashCode()), '', '', 'readback_num5'));
        emitFail(out);
    }
}
