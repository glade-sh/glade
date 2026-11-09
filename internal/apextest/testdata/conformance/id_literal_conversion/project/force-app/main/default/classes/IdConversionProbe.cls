public class IdConversionProbe {
    static String assign(String text) {
        try { Id value = text; return 'ok:' + String.valueOf(value); }
        catch (Exception issue) { return issue.getTypeName() + ':' + issue.getMessage(); }
    }
    static String cast(String text) {
        try { Object boxed = text; Id value = (Id) boxed; return 'ok:' + String.valueOf(value); }
        catch (Exception issue) { return issue.getTypeName() + ':' + issue.getMessage(); }
    }
    static String viaValueOf(String text) {
        try { Id value = Id.valueOf(text); return 'ok:' + String.valueOf(value); }
        catch (Exception issue) { return issue.getTypeName() + ':' + issue.getMessage(); }
    }
    public class Holder {
        public Id held;
        public Holder(Id value) { held = value; }
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        List<String> samples = new List<String>{
            '000000000000000000', '000000000000000', '000000000000000AAA', '001000000000000000', '001000000000001AAA',
            '001000000000001AAB', '001000000000001ZZZ', '001000000000001000', '00100000000000aAAA', '001000000000001',
            '0015g00000AbCdE', '0015g00000AbCdEAAV', '0015g00000AbCdEAAS', '001Hs00000AbCdEFGH', 'a0E000000000001',
            '00000000000000A', '0010000000000010', '00100000000000', '001000000000001AA-', '001000000000-01'
        };
        Integer index = 0;
        for (String sample : samples) {
            result.put('s' + index, new Map<String,Object>{ 'in' => sample, 'valueOf' => viaValueOf(sample), 'assign' => assign(sample), 'cast' => cast(sample) });
            index++;
        }
        try {
            Map<Id,String> byId = new Map<Id,String>();
            byId.put('000000000000000000', 'zero');
            result.put('mapGetLiteral', byId.get('000000000000000000'));
            result.put('mapContainsLiteral', byId.containsKey('000000000000000000'));
            result.put('mapKeyText', String.valueOf(new List<Id>(byId.keySet())[0]));
        } catch (Exception issue) { result.put('mapZero', issue.getTypeName() + ':' + issue.getMessage()); }
        try {
            Holder holder = new Holder('000000000000000000');
            result.put('holder', String.valueOf(holder.held));
            result.put('holderEqualsLiteral', holder.held == '000000000000000000');
            result.put('holderEqualsNull', holder.held == null);
        } catch (Exception issue) { result.put('holderZero', issue.getTypeName() + ':' + issue.getMessage()); }
        try {
            Id zero = '000000000000000000';
            result.put('zeroLength', String.valueOf(zero).length());
            result.put('zeroEqualsNull', zero == null);
        } catch (Exception issue) { result.put('zeroUse', issue.getTypeName() + ':' + issue.getMessage()); }
        return result;
    }
}
