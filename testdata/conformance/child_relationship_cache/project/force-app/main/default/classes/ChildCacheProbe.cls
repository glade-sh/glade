public class ChildCacheProbe {
    public static String observe() {
        String payload = '{"Contacts":{"totalSize":1,"done":true,"records":[{"attributes":{"type":"Contact"},"LastName":"Able"}]}}';
        Account first = (Account)JSON.deserialize(payload, Account.class);
        String serialized = JSON.serialize(first);
        Account roundtrip = (Account)JSON.deserialize(serialized, Account.class);
        Map<String,Object> result = new Map<String,Object>{'serialized' => serialized, 'roundtrip' => JSON.serialize(roundtrip)};
        return JSON.serialize(result);
    }
}
