// Item 3: JSON.deserialize, deserializeUntyped and JSONParser over escaped and raw lone surrogates.
String hi = String.fromCharArray(new List<Integer>{55357});
String lo = String.fromCharArray(new List<Integer>{56832});
Map<String, String> inputs = new Map<String, String>{
    'esc_hi' => '"\\uD800"',
    'esc_hi2' => '"\\uD83D"',
    'esc_lo' => '"\\uDC00"',
    'esc_pair' => '"\\uD83D\\uDE00"',
    'esc_hi_text' => '"\\uD83Dabc"',
    'esc_lo_hi' => '"\\uDE00\\uD83D"',
    'esc_hi_esc_a' => '"\\uD83D\\u0061"',
    'raw_hi' => '"' + hi + '"',
    'raw_lo' => '"' + lo + '"',
    'raw_hi_text' => '"' + hi + 'abc"',
    'raw_lo_hi' => '"' + lo + hi + '"',
    'raw_hi_esc_lo' => '"' + hi + '\\uDE00"',
    'esc_hi_raw_lo' => '"\\uD83D' + lo + '"'
};
Map<String, Object> o = new Map<String, Object>();
for (String k : inputs.keySet()) {
    String inp = inputs.get(k);
    Map<String, Object> r = new Map<String, Object>();
    try {
        String v = (String) JSON.deserialize(inp, String.class);
        r.put('deserialize', v == null ? null : v.getChars());
        r.put('reserialize', JSON.serialize(v).getChars());
    } catch (Exception e) { r.put('deserialize', 'ERR:' + e.getTypeName() + '|' + e.getMessage()); }
    try {
        Map<String, Object> m = (Map<String, Object>) JSON.deserializeUntyped('{"k":' + inp + '}');
        String v = (String) m.get('k');
        r.put('untyped', v == null ? null : v.getChars());
    } catch (Exception e) { r.put('untyped', 'ERR:' + e.getTypeName() + '|' + e.getMessage()); }
    try {
        List<String> l = (List<String>) JSON.deserialize('[' + inp + ']', List<String>.class);
        r.put('list', l[0].getChars());
    } catch (Exception e) { r.put('list', 'ERR:' + e.getTypeName() + '|' + e.getMessage()); }
    try {
        JSONParser p = JSON.createParser(inp);
        p.nextToken();
        r.put('parser', p.getText().getChars());
    } catch (Exception e) { r.put('parser', 'ERR:' + e.getTypeName() + '|' + e.getMessage()); }
    try {
        JSONParser p = JSON.createParser('{"k":' + inp + '}');
        p.nextToken(); p.nextToken(); p.nextToken();
        r.put('parser_field', p.getText().getChars());
    } catch (Exception e) { r.put('parser_field', 'ERR:' + e.getTypeName() + '|' + e.getMessage()); }
    o.put(k, r);
}
System.debug('CAP|json|' + JSON.serialize(o));
