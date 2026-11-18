// leading comment
/* block comment
   spanning lines */
public with sharing class CvShapeB {

    public static final Integer FIXED_VALUE = 12;
    public static Integer counter;
    private Integer inst = 3;
    private String label = 'x'
        + 'y';

    public CvShapeB() {
        inst = 4;
    }

    public enum Color { RED, GREEN,
        BLUE }

    public interface Shape {
        Integer area();
    }

    public class Sq implements Shape {
        private Integer s;
        public Sq(Integer s) { this.s = s; }
        public Integer area() { return s * s; }
    }

    @TestVisible
    private static Integer annotated(Integer a) { // trailing comment
        return a + 1; /* trailing block */
    }

    public static Integer loops(Integer n) {
        Integer total = 0;
        for (Integer i = 0;
             i < n;
             i++) {
            total += i;
        }
        for (Integer v : new List<Integer>{1, 2, 3}) {
            total += v;
        }
        Integer k = 0;
        while (k < 2) {
            k++;
        }
        do {
            k--;
        } while (k > 0);
        return total;
    }

    public static String sw(Integer x) {
        switch on x {
            when 1 {
                return 'one';
            }
            when 2, 3 {
                return 'few';
            }
            when else {
                return 'many';
            }
        }
    }

    public static Integer tern(Boolean b) {
        Integer r = b
            ? 1
            : 2;
        return r;
    }

    public static Integer guarded(Integer v) {
        Integer out = 0;
        try {
            out = 10 / v;
        } catch (MathException e) {
            out = -1;
        } finally {
            out += 1;
        }
        return out;
    }

    public static Integer listing() {
        List<Integer> items = new List<Integer>{
            1,
            2,
            3
        };
        Map<String, Integer> m = new Map<String, Integer>{
            'a' => 1,
            'b' => 2
        };
        return items.size() + m.size();
    }

    public static Integer query() {
        List<Account> accts = [
            SELECT Id
            FROM Account
            LIMIT 5
        ];
        return accts.size();
    }

    public static void neverCalled() {
        Integer z = 1;
        z++;
    }

    public Integer instanceCalled() {
        return inst;
    }

    public static Integer oneLiner() { return 1; }

    public static Integer unusedOneLiner() { return 2; }

    public static Integer constOnly() {
        return FIXED_VALUE;
    }
}
