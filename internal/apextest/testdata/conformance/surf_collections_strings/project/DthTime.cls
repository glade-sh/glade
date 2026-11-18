@IsTest
private class DthTime {
    @IsTest static void T_t01() {
        String p = 'row=t01';
        Time t = Time.newInstance(0, 0, 0, 0);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_t02() {
        String p = 'row=t02';
        Time t = Time.newInstance(0, 0, 0, 1);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_t03() {
        String p = 'row=t03';
        Time t = Time.newInstance(0, 0, 1, 0);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_t04() {
        String p = 'row=t04';
        Time t = Time.newInstance(12, 34, 56, 789);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_t05() {
        String p = 'row=t05';
        Time t = Time.newInstance(23, 59, 59, 999);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_t06() {
        String p = 'row=t06';
        Time t = Time.newInstance(1, 2, 3, 4);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_t07() {
        String p = 'row=t07';
        Time t = Time.newInstance(6, 30, 0, 0);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_t08() {
        String p = 'row=t08';
        Time t = Time.newInstance(12, 0, 0, 0);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_t09() {
        String p = 'row=t09';
        Time t = Time.newInstance(18, 45, 12, 345);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
    @IsTest static void T_t10() {
        String p = 'row=t10';
        Time t = Time.newInstance(23, 0, 0, 1);
        p += '|T=' + t.hashCode() + ';' + t.hour() + ';' + t.minute() + ';' + t.second() + ';' + t.millisecond() + ';' + System.hashCode(t);
        List<Time> l = new List<Time>{ t };
        p += '|L=' + l.hashCode() + ';' + l.size() + ';' + l.contains(t);
        Set<Time> s = new Set<Time>{ t };
        p += '|S=' + s.hashCode() + ';' + s.size() + ';' + s.contains(t);
        System.assert(false, p);
    }
}
