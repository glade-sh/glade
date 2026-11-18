public class CovTryCatch {
    public static Integer safeDivide(Integer a, Integer b) {
        Integer result = 0;
        try {
            result = a / b;
        } catch (MathException e) {
            result = -1;
        } finally {
            result = result + 0;
        }
        return result;
    }
}
