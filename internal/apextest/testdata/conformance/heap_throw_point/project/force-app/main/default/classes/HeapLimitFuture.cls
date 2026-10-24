public class HeapLimitFuture {
    @future
    public static void grow(String kind, Integer target) {
        HeapLimitProbe.grow(kind, target);
    }
}
