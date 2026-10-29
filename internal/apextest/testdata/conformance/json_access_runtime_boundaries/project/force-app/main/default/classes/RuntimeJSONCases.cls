public class RuntimeJSONCases {
    public static Map<String, Object> observe() {
        Map<String, Object> result = new Map<String, Object>();
        try { JSON.deserialize('{}', RuntimeJSONPayloads.Blocked.class); result.put('blockedDecode', 'allowed'); }
        catch (Exception e) { result.put('blockedDecode', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserializeStrict('{}', RuntimeJSONPayloads.EncodeOnly.class); result.put('strictDecode', 'allowed'); }
        catch (Exception e) { result.put('strictDecode', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.deserialize('[{}]', List<RuntimeJSONPayloads.Blocked>.class); result.put('listDecode', 'allowed'); }
        catch (Exception e) { result.put('listDecode', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.serialize(new RuntimeJSONPayloads.Blocked()); result.put('blockedEncode', 'allowed'); }
        catch (Exception e) { result.put('blockedEncode', e.getTypeName() + '|' + e.getMessage()); }
        try { JSON.serialize(new List<RuntimeJSONPayloads.DecodeOnly>{new RuntimeJSONPayloads.DecodeOnly()}); result.put('listEncode', 'allowed'); }
        catch (Exception e) { result.put('listEncode', e.getTypeName() + '|' + e.getMessage()); }
        result.put('nullDecode', JSON.deserialize('null', RuntimeJSONPayloads.Blocked.class) == null);
        result.put('allowedDecode', ((RuntimeJSONPayloads.DecodeOnly)JSON.deserialize('{"value":"decoded"}', RuntimeJSONPayloads.DecodeOnly.class)).value);
        result.put('allowedEncode', JSON.serialize(new RuntimeJSONPayloads.EncodeOnly()));
        result.put('sameClass', RuntimeJSONSelfBlocked.observeSelf());
        result.put('sameClassInstance', new RuntimeJSONSelfBlocked().observeInstance());
        result.put('sameClassOtherInstance', new RuntimeJSONSelfBlocked().observeOtherInstance());
        return result;
    }
}
