public class NowPairProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        Integer same = 0; Integer plusOne = 0; Integer more = 0;
        for (Integer i = 0; i < 300; i++) {
            Datetime a = System.now();
            Datetime b = System.now();
            Long d = b.getTime() - a.getTime();
            if (d == 0) { same++; } else if (d == 1) { plusOne++; } else { more++; }
        }
        m.put('sameInSameStatementPairs', same);
        m.put('plusOne', plusOne);
        m.put('more', more);
        Integer sameMax = 0; Integer plusOneC = 0;
        for (Integer i = 0; i < 300; i++) {
            Datetime a = Datetime.now();
            Datetime b = Datetime.now();
            Long d = b.getTime() - a.getTime();
            if (d == 0) { sameMax++; } else if (d == 1) { plusOneC++; }
        }
        m.put('datetimeNowSame', sameMax);
        m.put('datetimeNowPlusOne', plusOneC);
        Datetime x = System.now();
        Datetime y = System.now().addHours(36);
        m.put('endMinusStartMsIsExact36h', (y.getTime() - x.getTime()) == 36L * 3600000L);
        Long t1 = System.currentTimeMillis(); Long t2 = System.currentTimeMillis();
        m.put('currentTimeMillisSame', t1 == t2);
        Set<Long> distinct = new Set<Long>();
        for (Integer i = 0; i < 2000; i++) { distinct.add(System.now().getTime()); }
        m.put('distinctMsIn2000Reads', distinct.size() <= 1 ? 1 : (distinct.size() < 200 ? 2 : 3));
        Long first = System.now().getTime();
        Long last = first;
        for (Integer i = 0; i < 20000; i++) { last = System.now().getTime(); }
        Long elapsed = last - first;
        m.put('readsPerMsBucket', elapsed == 0 ? 'zero' : (20000 / elapsed >= 100 ? 'ge100' : (20000 / elapsed >= 10 ? 'ge10' : 'lt10')));
        m.put('rawElapsedMs20000', elapsed);
        return m;
    }
}
