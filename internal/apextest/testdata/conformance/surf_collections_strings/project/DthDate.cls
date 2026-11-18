@IsTest
private class DthDate {
    @IsTest static void T_d01() {
        String p = 'row=d01';
        Date t = Date.newInstance(1900, 1, 1);
        p += '|D=' + t.hashCode() + ';' + t.year() + ';' + t.month() + ';' + t.day() + ';' + System.hashCode(t);
        List<Date> l = new List<Date>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_d02() {
        String p = 'row=d02';
        Date t = Date.newInstance(1969, 12, 31);
        p += '|D=' + t.hashCode() + ';' + t.year() + ';' + t.month() + ';' + t.day() + ';' + System.hashCode(t);
        List<Date> l = new List<Date>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_d03() {
        String p = 'row=d03';
        Date t = Date.newInstance(1970, 1, 1);
        p += '|D=' + t.hashCode() + ';' + t.year() + ';' + t.month() + ';' + t.day() + ';' + System.hashCode(t);
        List<Date> l = new List<Date>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_d04() {
        String p = 'row=d04';
        Date t = Date.newInstance(2026, 11, 16);
        p += '|D=' + t.hashCode() + ';' + t.year() + ';' + t.month() + ';' + t.day() + ';' + System.hashCode(t);
        List<Date> l = new List<Date>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_d05() {
        String p = 'row=d05';
        Date t = Date.newInstance(4000, 12, 31);
        p += '|D=' + t.hashCode() + ';' + t.year() + ';' + t.month() + ';' + t.day() + ';' + System.hashCode(t);
        List<Date> l = new List<Date>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        System.assert(false, p);
    }
}
