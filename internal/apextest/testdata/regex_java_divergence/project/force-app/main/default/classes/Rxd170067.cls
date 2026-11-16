@IsTest
private class Rxd170067 {
    @IsTest static void d17_x_look() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)(? =a)a').matcher('a'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d17_x_spacecolon() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)(? :a)').matcher('a'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d17_x_namedref() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)( ?<n>a)\\k<n>').matcher('aa'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d17_x_classcr() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)[a#c\rb]').matcher('b'); Boolean f = m.find(); observation = 'find ' + f + ' groups ' + m.groupCount(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
