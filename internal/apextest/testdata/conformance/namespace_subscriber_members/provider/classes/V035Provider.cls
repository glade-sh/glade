global virtual class V035Provider {
    global class GlobalInner extends V035Provider { global GlobalInner() {} global Integer size() { return 2; } }
    public class PublicInner extends V035Provider { public PublicInner() {} }
    private class PrivateInner extends V035Provider { private PrivateInner() {} }
    global class GlobalPrivateCtor extends V035Provider { private GlobalPrivateCtor() {} }
    global static String attempt(Type t) { if (t == null) return 'null'; try { t.newInstance(); return 'constructed'; } catch (Exception e) { return e.getTypeName(); } }
    global static String ownView() { return attempt(PrivateInner.class) + '|' + attempt(PublicInner.class) + '|' + attempt(GlobalInner.class) + '|' + attempt(GlobalPrivateCtor.class); }
    global static Integer touch() { new PrivateInner(); new PublicInner(); new GlobalInner(); new GlobalPrivateCtor(); return 4; }
}
