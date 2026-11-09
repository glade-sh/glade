public class ShadowProbe {
    public static Map<String, Object> observe() {
        Map<String, Object> r = ShadowHost.observe();
        r.putAll(ShadowOther.observe());
        return r;
    }
}
