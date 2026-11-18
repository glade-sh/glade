@IsTest
private class DecHashProbe {
    static String row(String id, String fld, String lit, String phase, String s, String p, String sc,
                      String h, String lh, String ss, String con, String notes) {
        List<String> cols = new List<String>{ id, fld, lit, phase, s, p, sc, h, lh, ss, con, notes };
        return String.join(cols, '\t');
    }

    static String rowD(String id, String fld, String lit, String phase, Decimal d, Integer lh,
                       Integer ss, String con, String notes) {
        String s = '';
        String p = '';
        String sc = '';
        String h = '';
        if (d != null) {
            s = String.valueOf(d);
            p = String.valueOf(d.precision());
            sc = String.valueOf(d.scale());
            h = String.valueOf(d.hashCode());
        }
        return row(id, fld, lit, phase, s, p, sc, h,
            lh == null ? '' : String.valueOf(lh),
            ss == null ? '' : String.valueOf(ss),
            con == null ? '' : con,
            notes == null ? '' : notes);
    }

    static void emitFail(List<String> lines) {
        lines.add('END');
        System.assert(false, '\n' + String.join(lines, '\n'));
    }

    static void setF(Dec_Probe__c r, String f, Decimal v) {
        if (f == 'Curr__c') { r.Curr__c = v; }
        else if (f == 'Pct__c') { r.Pct__c = v; }
        else if (f == 'Num0__c') { r.Num0__c = v; }
        else if (f == 'Num5__c') { r.Num5__c = v; }
        else { r.Num8__c = v; }
    }

    static Decimal getF(Dec_Probe__c r, String f) {
        if (f == 'Curr__c') { return r.Curr__c; }
        if (f == 'Pct__c') { return r.Pct__c; }
        if (f == 'Num0__c') { return r.Num0__c; }
        if (f == 'Num5__c') { return r.Num5__c; }
        return r.Num8__c;
    }

    static String runField(String id, String fld, String lit) {
        List<String> out = new List<String>();
        Decimal v;
        try {
            v = Decimal.valueOf(lit);
        } catch (Exception e) {
            out.add(row(id, fld, lit, 'valueOf_error', '', '', '', '', '', '', '',
                String.valueOf(e.getTypeName()) + ': ' + String.valueOf(e.getMessage()).replace('\n', ' ')));
            return String.join(out, '\n');
        }
        Dec_Probe__c rec = new Dec_Probe__c(Label__c = id + ' ' + fld);
        setF(rec, fld, v);
        Decimal mem = getF(rec, fld);
        Integer lh = (new List<Decimal>{ v }).hashCode();
        Set<Decimal> st = new Set<Decimal>{ v };
        String con = String.valueOf(st.contains(v.setScale(v.scale() + 2)));
        out.add(rowD(id, fld, lit, 'memory', mem, lh, st.size(), con, 'assigned_literal'));
        out.add(rowD(id, fld, lit, 'direct', v, lh, st.size(), con, 'bare_literal'));
        try {
            insert rec;
            out.add(row(id, fld, lit, 'dml', '', '', '', '', '', '', '', 'OK'));
            Dec_Probe__c rb = [SELECT Curr__c, Pct__c, Num0__c, Num5__c, Num8__c
                               FROM Dec_Probe__c WHERE Id = :rec.Id];
            Decimal got = getF(rb, fld);
            Integer gl = got == null ? null : (new List<Decimal>{ got }).hashCode();
            out.add(rowD(id, fld, lit, 'readback', got, gl, null, null, 'soql_all_five'));
        } catch (DmlException e) {
            String msg = String.valueOf(e.getDmlType(0)) + ': ' + String.valueOf(e.getDmlMessage(0));
            msg = msg.replace('\n', ' ');
            out.add(row(id, fld, lit, 'dml', '', '', '', '', '', '', '', 'FAIL ' + msg));
        }
        return String.join(out, '\n');
    }

    static String fieldCase(String id, String fld, String lit) {
        return runField(id, fld, lit);
    }
    @IsTest static void F01() {
        emitFail(new List<String>{ fieldCase('F01', 'Curr__c', '1.5') });
    }

    @IsTest static void F02() {
        emitFail(new List<String>{ fieldCase('F02', 'Pct__c', '1.5') });
    }

    @IsTest static void F03() {
        emitFail(new List<String>{ fieldCase('F03', 'Num0__c', '1.5') });
    }

    @IsTest static void F04() {
        emitFail(new List<String>{ fieldCase('F04', 'Num5__c', '1.5') });
    }

    @IsTest static void F05() {
        emitFail(new List<String>{ fieldCase('F05', 'Num8__c', '1.5') });
    }

    @IsTest static void F06() {
        emitFail(new List<String>{ fieldCase('F06', 'Curr__c', '0') });
    }

    @IsTest static void F07() {
        emitFail(new List<String>{ fieldCase('F07', 'Pct__c', '0') });
    }

    @IsTest static void F08() {
        emitFail(new List<String>{ fieldCase('F08', 'Num0__c', '0') });
    }

    @IsTest static void F09() {
        emitFail(new List<String>{ fieldCase('F09', 'Num5__c', '0') });
    }

    @IsTest static void F10() {
        emitFail(new List<String>{ fieldCase('F10', 'Num8__c', '0') });
    }

    @IsTest static void F11() {
        emitFail(new List<String>{ fieldCase('F11', 'Curr__c', '-0.01') });
    }

    @IsTest static void F12() {
        emitFail(new List<String>{ fieldCase('F12', 'Pct__c', '-0.01') });
    }

    @IsTest static void F13() {
        emitFail(new List<String>{ fieldCase('F13', 'Num0__c', '-0.01') });
    }

    @IsTest static void F14() {
        emitFail(new List<String>{ fieldCase('F14', 'Num5__c', '-0.01') });
    }

    @IsTest static void F15() {
        emitFail(new List<String>{ fieldCase('F15', 'Num8__c', '-0.01') });
    }

    @IsTest static void F16() {
        emitFail(new List<String>{ fieldCase('F16', 'Curr__c', '123456.789') });
    }

    @IsTest static void F17() {
        emitFail(new List<String>{ fieldCase('F17', 'Pct__c', '123456.789') });
    }

    @IsTest static void F18() {
        emitFail(new List<String>{ fieldCase('F18', 'Num0__c', '123456.789') });
    }

    @IsTest static void F19() {
        emitFail(new List<String>{ fieldCase('F19', 'Num5__c', '123456.789') });
    }

    @IsTest static void F20() {
        emitFail(new List<String>{ fieldCase('F20', 'Num8__c', '123456.789') });
    }

    @IsTest static void F21() {
        emitFail(new List<String>{ fieldCase('F21', 'Curr__c', '1E-8') });
    }

    @IsTest static void F22() {
        emitFail(new List<String>{ fieldCase('F22', 'Pct__c', '1E-8') });
    }

    @IsTest static void F23() {
        emitFail(new List<String>{ fieldCase('F23', 'Num0__c', '1E-8') });
    }

    @IsTest static void F24() {
        emitFail(new List<String>{ fieldCase('F24', 'Num5__c', '1E-8') });
    }

    @IsTest static void F25() {
        emitFail(new List<String>{ fieldCase('F25', 'Num8__c', '1E-8') });
    }

    @IsTest static void S01() {
        List<String> out = new List<String>();
        Decimal b = Decimal.valueOf('1.5');
        for (Integer s = 3; s <= 8; s++) {
            Decimal d = b.setScale(s);
            out.add(rowD('S01', 'Decimal', '1.5', 'setScale' + String.valueOf(s), d,
                (new List<Decimal>{ d }).hashCode(), (new Set<Decimal>{ d }).size(), null,
                'source_prec=' + b.precision() + ';source_scale=' + b.scale()));
        }
        emitFail(out);
    }

    @IsTest static void S02() {
        List<String> out = new List<String>();
        for (String lit : new List<String>{ '1.5', '1.50', '1.500', '1.5000' }) {
            Decimal d = Decimal.valueOf(lit);
            out.add(rowD('S02', 'Decimal', lit, 'valueOf', d, (new List<Decimal>{ d }).hashCode(),
                null, null, 'Decimal.valueOf'));
        }
        emitFail(out);
    }

    @IsTest static void S03() {
        List<String> out = new List<String>();
        for (String lit : new List<String>{ '1.500', '1.5000', '0.0100' }) {
            Decimal d = Decimal.valueOf(lit);
            out.add(rowD('S03', 'Decimal', lit, 'strip_before', d,
                (new List<Decimal>{ d }).hashCode(), null, null, 'raw'));
            Decimal t = d.stripTrailingZeros();
            out.add(rowD('S03', 'Decimal', lit, 'strip_after', t,
                (new List<Decimal>{ t }).hashCode(), null, null, 'stripped'));
        }
        emitFail(out);
    }

    @IsTest static void S04() {
        List<String> out = new List<String>();
        Decimal d = 5L;
        out.add(rowD('S04', 'Decimal', '5L', 'arith_init', d, (new List<Decimal>{ d }).hashCode(),
            null, null, 'Decimal d = 5L'));
        d = d + 1;
        out.add(rowD('S04', 'Decimal', '5L+1', 'arith_plus1', d, (new List<Decimal>{ d }).hashCode(),
            null, null, 'd = d + 1'));
        d = d / 2;
        out.add(rowD('S04', 'Decimal', '(5L+1)/2', 'arith_div2', d, (new List<Decimal>{ d }).hashCode(),
            null, null, 'd = d / 2'));
        d = 3L * 2;
        out.add(rowD('S04', 'Decimal', '3L*2', 'arith_mul', d, (new List<Decimal>{ d }).hashCode(),
            null, null, 'd = 3L * 2'));
        emitFail(out);
    }

    @IsTest static void S05() {
        List<String> out = new List<String>();
        Decimal d = 0.1 + 0.2;
        out.add(rowD('S05', 'Decimal', '0.1+0.2', 'decimal_literal_sum', d,
            (new List<Decimal>{ d }).hashCode(), null, null, 'both operands decimal literals'));
        emitFail(out);
    }

    @IsTest static void S11a() {
        List<String> out = new List<String>();
        Decimal v = Decimal.valueOf('1.5');
        Dec_Probe__c r = new Dec_Probe__c(Label__c = 'S11a', Curr__c = v, Pct__c = v,
            Num0__c = v, Num5__c = v, Num8__c = v);
        insert r;
        Dec_Probe__c rb = [SELECT Curr__c, Pct__c, Num0__c, Num5__c, Num8__c
                           FROM Dec_Probe__c WHERE Id = :r.Id];
        for (String f : new List<String>{ 'Curr__c', 'Pct__c', 'Num0__c', 'Num5__c', 'Num8__c' }) {
            Decimal got = getF(rb, f);
            out.add(rowD('S11a', f, '1.5', 'combined_readback', got,
                (new List<Decimal>{ got }).hashCode(), null, null, 'one_record_all_five_fields'));
        }
        emitFail(out);
    }

    @IsTest static void S11b() {
        List<String> out = new List<String>();
        Decimal v = Decimal.valueOf('0');
        Dec_Probe__c r = new Dec_Probe__c(Label__c = 'S11b', Curr__c = v, Pct__c = v,
            Num0__c = v, Num5__c = v, Num8__c = v);
        insert r;
        Dec_Probe__c rb = [SELECT Curr__c, Pct__c, Num0__c, Num5__c, Num8__c
                           FROM Dec_Probe__c WHERE Id = :r.Id];
        for (String f : new List<String>{ 'Curr__c', 'Pct__c', 'Num0__c', 'Num5__c', 'Num8__c' }) {
            Decimal got = getF(rb, f);
            out.add(rowD('S11b', f, '0', 'combined_readback', got,
                (new List<Decimal>{ got }).hashCode(), null, null, 'one_record_all_five_fields'));
        }
        emitFail(out);
    }
}
