// N24: two-digit-year century pivot for Date.parse and Datetime.parse in the
// running user's locale. The layout is taken from format() so the script works
// for any locale: the year 2008 in the formatted text is replaced by yy.
String locale = UserInfo.getLocale();
Date d = Date.newInstance(2008, 1, 28);
Datetime dt = Datetime.newInstance(2008, 1, 28, 15, 4, 0);
String df = d.format();
String dtf = dt.format();
Map<String,Object> env = new Map<String,Object>{'locale' => locale, 'tz' => UserInfo.getTimeZone().getID(),
    'date_format' => df, 'datetime_format' => dtf, 'dt_chars' => dtf.getChars()};
try { env.put('date_roundtrip', String.valueOf(Date.parse(df))); } catch (Exception e) { env.put('date_roundtrip', e.getTypeName() + ': ' + e.getMessage()); }
try { env.put('datetime_roundtrip', Datetime.parse(dtf).formatGmt('yyyy-MM-dd HH:mm:ss')); } catch (Exception e) { env.put('datetime_roundtrip', e.getTypeName() + ': ' + e.getMessage()); }
System.debug('CAP|n24.env|' + JSON.serialize(env));

String dateOut = '';
String dtOut = '';
for (Integer yy = 0; yy < 100; yy++) {
    String two = (yy < 10 ? '0' : '') + String.valueOf(yy);
    String input = df.replace('2008', two);
    String r;
    try { r = String.valueOf(Date.parse(input)); } catch (Exception e) { r = 'ERR:' + e.getTypeName(); }
    dateOut += two + '=' + r + ';';
    String dinput = dtf.replace('2008', two);
    try { r = Datetime.parse(dinput).formatGmt('yyyy-MM-dd HH:mm'); } catch (Exception e) { r = 'ERR:' + e.getTypeName(); }
    dtOut += two + '=' + r + ';';
}
System.debug('CAP|n24.date2|' + dateOut);
System.debug('CAP|n24.datetime2|' + dtOut);

// Other year widths: one digit, three digits, zero-padded four digits, five digits.
String extra = '';
for (String y : new List<String>{'0', '5', '9', '100', '190', '090', '999', '0090', '0008', '2008', '20088', '10000'}) {
    String input = df.replace('2008', y);
    String r;
    try { r = String.valueOf(Date.parse(input)); } catch (Exception e) { r = 'ERR:' + e.getTypeName(); }
    extra += y + '=' + r + ';';
    String dinput = dtf.replace('2008', y);
    try { r = Datetime.parse(dinput).formatGmt('yyyy-MM-dd HH:mm'); } catch (Exception e) { r = 'ERR:' + e.getTypeName(); }
    extra += 'dt:' + y + '=' + r + ';';
}
System.debug('CAP|n24.widths|' + extra);

// Datetime.parse layout candidates for this locale (which punctuation and am/pm forms parse).
String cands = '';
for (String c : new List<String>{'2008-01-28, 3:04 PM', '2008-01-28 3:04 PM', '2008-01-28, 3:04 p.m.', '2008-01-28 3:04 p.m.',
        '2008-01-28, 15:04', '2008-01-28 15:04', '1/28/2008, 3:04 PM', '1/28/2008 3:04 PM', '2008-1-28, 3:04 p.m.', '2008-1-28 3:04 p.m.',
        '28/01/2008, 15:04', '2008-01-28, 3:04 pm', '2008-01-28, 3:04 PM.', '2008-01-28, 03:04 p.m.'}) {
    String r;
    try { r = Datetime.parse(c).formatGmt('yyyy-MM-dd HH:mm'); } catch (Exception e) { r = 'ERR:' + e.getTypeName(); }
    cands += c + '=' + r + ';';
}
System.debug('CAP|n24.dtlayouts|' + cands);
String dcands = '';
for (String c : new List<String>{'2008-01-28', '2008-1-28', '08-01-28', '8-1-28', '1/28/2008', '1/28/08', '28/01/2008', '2008/01/28', '20080128'}) {
    String r;
    try { r = String.valueOf(Date.parse(c)); } catch (Exception e) { r = 'ERR:' + e.getTypeName(); }
    dcands += c + '=' + r + ';';
}
System.debug('CAP|n24.dlayouts|' + dcands);
