@IsTest
private class Rxd230062 {
    @IsTest static void d22_name_bell_emoji() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{BELL}').matcher('\uD83D\uDD14'); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d22_name_bell_emoji_len() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{BELL}').matcher('x\uD83D\uDD14y'); Boolean f = m.find(); observation = 'find ' + f + ' start ' + (f ? String.valueOf(m.start()) : '-') + ' len ' + '\uD83D\uDD14'.length(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
