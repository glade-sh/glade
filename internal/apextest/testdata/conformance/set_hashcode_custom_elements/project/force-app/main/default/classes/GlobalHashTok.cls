global class GlobalHashTok {
    global String a;
    global Integer extra;
    global GlobalHashTok(String a, Integer extra) { this.a = a; this.extra = extra; }
    global Boolean equals(Object o) { return o instanceof GlobalHashTok && ((GlobalHashTok) o).a == a; }
    global Integer hashCode() { return a.hashCode(); }
}
