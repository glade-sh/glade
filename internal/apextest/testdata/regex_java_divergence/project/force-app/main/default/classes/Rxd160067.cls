@IsTest
private class Rxd160067 {
    @IsTest static void d16_x_cr() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)a #c\r|b').matcher('b'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d16_x_ls() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)a #c\u2028|b').matcher('b'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d16_xd_cr() {
        String observation;
        try { Matcher m = Pattern.compile('(?xd)a #c\r|b').matcher('b'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d16_x_spacegrp() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)( ?:a)').matcher('a'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d16_x_octal() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)\\0 1 4 2').matcher('b'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d16_x_hex() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)\\x2 3').matcher('#'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
