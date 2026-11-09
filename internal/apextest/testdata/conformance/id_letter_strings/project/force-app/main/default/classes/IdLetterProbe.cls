public class IdLetterProbe {
    static String bad(String s, Exception e) {
        return (e.getTypeName() == 'System.StringException' && e.getMessage() == 'Invalid id: ' + s) ? 'X' : e.getTypeName() + ':' + e.getMessage();
    }
    static String one(String s) {
        List<String> r = new List<String>();
        try { Id x = s; r.add('' + x); } catch (Exception e) { r.add(bad(s, e)); }
        try { Id x = Id.valueOf(s); r.add('' + x); } catch (Exception e) { r.add(bad(s, e)); }
        try { Id x = (Id) s; r.add('' + x); } catch (Exception e) { r.add(bad(s, e)); }
        return String.join(r, ' ~ ');
    }
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        List<String> inputs = new List<String>{
            'abcdefghijklmno', 'ABCDEFGHIJKLMNO', '999000000000000', 'abc000000000000', '001abcdefghijkl',
            'zzzzzzzzzzzzzzz', 'a0E000000000001', 'abcdefghijklmnopqr', 'ABCDEFGHIJKLMNOPQR', '001000000000001xyz',
            '001ABCDEFGHIJKL', '0010000000000AB', '00100000000000_', '001 00000000001', '001000000000001-',
            '12345678901234A', '000000000000001', '00Q000000000001', 'x', 'abcdefghijklmn', 'abcdefghijklmnop',
            'abcde0000000000', 'abcdef000000000', '001ab0cdefghijk', '001ab0000ZZZZZZ', '001ab1000000000', '001ab0000000000AAA',
            '001abc000000000AAA', '001ab0000000001AAA', '00100000000000a', '001000000000zzz', '0010000000zzzzz', '001000zzzzzzzzz'
        };
        for (Integer i = 0; i < inputs.size(); i++) { o.put('s' + i + '=' + inputs[i], one(inputs[i])); }
        // one position at a time over two bases; a=letter, 5=non-zero digit, Z=capital, _=punctuation
        List<String> bases = new List<String>{ '001000000000000', 'a0E000000000000', '001ab0000000000' };
        for (String base : bases) {
            for (Integer pos = 0; pos < 15; pos++) {
                String res = '';
                for (String ch : new List<String>{ 'a', '5', 'Z', '_' }) {
                    String cand = base.substring(0, pos) + ch + base.substring(pos + 1);
                    try { Id x = cand; res += '1'; } catch (Exception e) { res += '0'; }
                }
                o.put('p' + base + '@' + pos, res);
            }
        }
        // the first six characters vary while the tail is zeros or letters
        List<String> heads = new List<String>{ '123456', '000000', 'abcdef', 'ABCDEF', '0a0a0a', 'a1b2c3', '999999', '00a00a', 'zzzzzz', '5g1h00' };
        for (String h : heads) {
            Boolean z, l, d;
            try { Id x = h + '000000000'; z = true; } catch (Exception e) { z = false; }
            try { Id x = h + 'ghijklmno'; l = true; } catch (Exception e) { l = false; }
            try { Id x = h + '123456789'; d = true; } catch (Exception e) { d = false; }
            o.put('h' + h, '' + z + '/' + l + '/' + d);
        }
        return o;
    }
}
