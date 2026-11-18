public class CvShapeC {
    public static Integer plainField = 5;
    public static final String LABEL = 'a'
        + 'b'
        + 'c';
    public static Integer sum;
    private static Map<String, Integer> lookup = new Map<String, Integer>{
        'one' => 1,
        'two' => 2
    };

    public CvShapeC() {
        this(1);
    }

    public CvShapeC(Integer start) {
        sum = start;
    }

    @AuraEnabled(cacheable=true)
    public static Integer annotatedMulti(
        Integer a,
        Integer b
    ) {
        return a
            + b;
    }

    public static Integer elseIfChain(Integer x) {
        if (x < 0) {
            return -1;
        } else if (x == 0) {
            return 0;
        } else if (x < 10) {
            return 1;
        } else {
            return 2;
        }
    }

    public static Integer nullSafe(Account a) {
        Integer n = a?.NumberOfEmployees ?? 7;
        return n;
    }

    public static void assertLine(Integer x) {
        System.assert(x > 0, 'positive');
        System.assertEquals(1, 1);
        Integer y = x;
    }

    public static String concatLiteral(String v) {
        String s = 'start-'
            + v
            + '-end';
        String t = 'x' + 'y';
        String u = 'p'
            + 'q';
        return s + t + u;
    }

    public static List<Account> soqlLoop() {
        List<Account> out = new List<Account>();
        for (Account a : [SELECT Id, Name
                          FROM Account
                          LIMIT 3]) {
            out.add(a);
        }
        return out;
    }

    public static void dmlAndThrow(Boolean fail) {
        Account a = new Account(
            Name = 'cv-shape-c'
        );
        insert a;
        if (fail) {
            throw new CvException(
                'boom'
            );
        }
        a.Name = 'cv-shape-c2';
        update a;
    }

    public static Integer ternaryNested(Integer x) {
        return x > 5
            ? (x > 10 ? 3 : 2)
            : 1;
    }

    public static Boolean shortCircuit(Boolean a, Boolean b) {
        return a
            && b
            || a;
    }

    public static Integer instanceOfCheck(Object o) {
        if (o instanceof String) {
            return 1;
        }
        return 0;
    }

    public static void emptyMethod() {
    }

    public static void voidReturn() {
        return;
    }

    public static Integer withFinal(final Integer v) {
        final Integer w = v;
        return w;
    }

    public static Integer neverCalledC() {
        return 99;
    }

    public class InnerC {
        public Integer field = 3;
        public Integer get() {
            return field;
        }
        public Integer unusedInner() {
            return 1;
        }
    }

    public Integer prop {
        get { return 1; }
        set { sum = value; }
    }

    public Integer propFull {
        get {
            return 2;
        }
        private set;
    }
}
