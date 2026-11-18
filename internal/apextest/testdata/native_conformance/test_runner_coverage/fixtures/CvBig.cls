public class CvBig {
    public static Integer used() {
        Integer a = 1;
        return a;
    }
    public static Integer unused1() {
        Integer a = 1;
        a += 1;
        a += 2;
        a += 3;
        return a;
    }
    public static Integer unused2() {
        Integer a = 1;
        a += 1;
        a += 2;
        a += 3;
        return a;
    }
    public static Integer unused3() {
        Integer a = 1;
        a += 1;
        a += 2;
        return a;
    }
}
