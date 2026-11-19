@IsTest
private class Rxd190067 {
    @IsTest static void d19_x_named() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)\\N{LATIN SMALL LETTER A}').matcher('a'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d19_x_class_named() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)[a b]\\N{LATIN SMALL LETTER A}').matcher('ba'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d19_named_unknown() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{FOO BAR}').matcher('a'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d19_x_named_unknown() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)\\N{FOO BAR}').matcher('a'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
