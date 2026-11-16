public class CvSetupCov {
    public static Integer state = 2;
    static {
        state += 3;
    }
    public static void makeRecord() {
        insert as system new Account(Name = 'setup-coverage-probe');
    }
    public static Integer value() {
        return 42;
    }
}
