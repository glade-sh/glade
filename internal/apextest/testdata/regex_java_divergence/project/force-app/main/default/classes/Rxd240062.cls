@IsTest
private class Rxd240062 {
    @IsTest static void d24_ctrl_astral() {
        String observation;
        try { Matcher m = Pattern.compile('\\c\uD83D\uDE00').matcher('x\uD83D\uDE40y'); Boolean f = m.find(); observation = 'find ' + f + ' start ' + (f ? String.valueOf(m.start()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d24_ctrl_ascii() {
        String observation;
        try { Matcher m = Pattern.compile('\\cA').matcher('x\u0001y'); Boolean f = m.find(); observation = 'find ' + f + ' start ' + (f ? String.valueOf(m.start()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
