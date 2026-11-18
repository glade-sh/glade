public class CvSetupStatic {
    public static Integer state = 2;
    static {
        state += 3;
    }
    public static Integer other() {
        return 1;
    }
}
