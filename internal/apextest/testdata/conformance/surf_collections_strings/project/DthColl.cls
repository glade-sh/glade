@IsTest
private class DthColl {
    @IsTest static void T_i01() {
        String p = 'row=i01';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(1970, 1, 1), Time.newInstance(0, 0, 0, 0));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1970-01-01 00:00:00.000');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i02() {
        String p = 'row=i02';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(1970, 1, 1), Time.newInstance(0, 0, 0, 1));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1970-01-01 00:00:00.001');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i03() {
        String p = 'row=i03';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(1970, 1, 1), Time.newInstance(0, 0, 0, 999));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1970-01-01 00:00:00.999');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i04() {
        String p = 'row=i04';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(1970, 1, 1), Time.newInstance(0, 0, 1, 0));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1970-01-01 00:00:01.000');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i05() {
        String p = 'row=i05';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(1969, 12, 31), Time.newInstance(23, 59, 59, 999));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1969-12-31 23:59:59.999');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i06() {
        String p = 'row=i06';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(1900, 1, 1), Time.newInstance(0, 0, 0, 0));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1900-01-01 00:00:00.000');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i07() {
        String p = 'row=i07';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(2038, 1, 19), Time.newInstance(3, 14, 8, 0));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('2038-01-19 03:14:08.000');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i08() {
        String p = 'row=i08';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(4000, 12, 31), Time.newInstance(0, 0, 0, 0));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('4000-12-31 00:00:00.000');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i09() {
        String p = 'row=i09';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(4000, 12, 31), Time.newInstance(23, 59, 59, 999));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('4000-12-31 23:59:59.999');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i10() {
        String p = 'row=i10';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(1905, 6, 15), Time.newInstance(8, 7, 6, 543));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1905-06-15 08:07:06.543');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i11() {
        String p = 'row=i11';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(1982, 1, 20), Time.newInstance(4, 2, 12, 415));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1982-01-20 04:02:12.415');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i12() {
        String p = 'row=i12';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(1993, 10, 4), Time.newInstance(5, 25, 48, 663));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1993-10-04 05:25:48.663');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i13() {
        String p = 'row=i13';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(2009, 12, 23), Time.newInstance(20, 12, 34, 14));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('2009-12-23 20:12:34.014');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i14() {
        String p = 'row=i14';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(2018, 5, 3), Time.newInstance(4, 42, 44, 705));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('2018-05-03 04:42:44.705');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i15() {
        String p = 'row=i15';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(2047, 6, 29), Time.newInstance(19, 59, 40, 644));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('2047-06-29 19:59:40.644');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i16() {
        String p = 'row=i16';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(2085, 1, 10), Time.newInstance(15, 18, 19, 962));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('2085-01-10 15:18:19.962');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i17() {
        String p = 'row=i17';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(2178, 7, 26), Time.newInstance(0, 4, 25, 534));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('2178-07-26 00:04:25.534');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i18() {
        String p = 'row=i18';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(2356, 6, 10), Time.newInstance(19, 53, 22, 366));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('2356-06-10 19:53:22.366');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i19() {
        String p = 'row=i19';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(2543, 5, 23), Time.newInstance(15, 15, 18, 853));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('2543-05-23 15:15:18.853');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i20() {
        String p = 'row=i20';
        try {
            Datetime a = Datetime.newInstanceGmt(
                Date.newInstance(3093, 11, 11), Time.newInstance(6, 40, 40, 38));
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('3093-11-11 06:40:40.038');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i21() {
        String p = 'row=i21';
        try {
            Datetime a = Datetime.newInstance(2147483647L);
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1970-01-25 20:31:23.647');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i22() {
        String p = 'row=i22';
        try {
            Datetime a = Datetime.newInstance(2147483648L);
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1970-01-25 20:31:23.648');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i23() {
        String p = 'row=i23';
        try {
            Datetime a = Datetime.newInstance(4294967295L);
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1970-02-19 17:02:47.295');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i24() {
        String p = 'row=i24';
        try {
            Datetime a = Datetime.newInstance(4294967296L);
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1970-02-19 17:02:47.296');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i25() {
        String p = 'row=i25';
        try {
            Datetime a = Datetime.newInstance(-4294967296L);
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1969-11-12 06:57:12.704');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i26() {
        String p = 'row=i26';
        try {
            Datetime a = Datetime.newInstance(-4294967297L);
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1969-11-12 06:57:12.703');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i27() {
        String p = 'row=i27';
        try {
            Datetime a = Datetime.newInstance(-12219292800001L);
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1582-10-14 23:59:59.999');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i28() {
        String p = 'row=i28';
        try {
            Datetime a = Datetime.newInstance(-12219292800000L);
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1582-10-15 00:00:00.000');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
    @IsTest static void T_i29() {
        String p = 'row=i29';
        try {
            Datetime a = Datetime.newInstance(-12219292799999L);
            p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
            Datetime b = Datetime.valueOfGmt('1582-10-15 00:00:00.001');
            p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
            List<Datetime> l = new List<Datetime>{ a, b };
            p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(a) + ';' + l.contains(b);
            Set<Datetime> s = new Set<Datetime>{ a, b };
            p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(a) + ';' + s.contains(b);
            p += '|EQ=' + (a == b) + ';' + (a.hashCode() == b.hashCode());
        } catch (Exception e) {
            p += '|EX=' + e.getTypeName() + ';' + e.getMessage();
        }
        System.assert(false, p);
    }
}
