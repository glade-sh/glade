@IsTest private class StringScanTest {
    private static void observe(String name, Map<String,Object> values) {
        System.assert(false, 'OBSERVED|' + name + '|' + JSON.serialize(values));
    }
    @IsTest static void scanAscii() {
        String text = 'abc|'.repeat(512) + '#end';
        Integer count = 0, position = 0;
        while (position < text.length()) {
            Integer found = text.indexOf('|', position);
            if (found < 0) break;
            count += text.charAt(found);
            position = found + 1;
        }
        observe('scanAscii', new Map<String,Object>{
            'sum' => count, 'length' => text.length(),
            'suffix' => text.substring(position),
            'emptyPastEnd' => text.indexOf('', text.length()+2),
            'negativeStart' => text.indexOf('abc', -3),
            'missing' => text.indexOf('none', 3)
        });
    }
    @IsTest static void unicodeControl() {
        String text = 'xΩ😀|end';
        observe('unicodeControl', new Map<String,Object>{
            'length' => text.length(), 'delimiter' => text.indexOf('|', 1),
            'bmp' => text.charAt(1), 'tail' => text.charAt(4),
            'suffix' => text.substring(4), 'negativeStart' => text.indexOf('x', -1)
        });
    }
    @IsTest static void boundsControl() {
        String text = 'abc';
        Map<String,Object> values = new Map<String,Object>();
        try { text.charAt(3); } catch (Exception e) { values.put('charEnd',e.getMessage()); }
        try { text.charAt(-1); } catch (Exception e) { values.put('charNegative',e.getMessage()); }
        try { text.substring(4); } catch (Exception e) { values.put('substringStart',e.getMessage()); }
        try { text.substring(2,1); } catch (Exception e) { values.put('substringReverse',e.getMessage()); }
        observe('boundsControl', values);
    }
}
