@IsTest
private class Rxd180062 {
    @IsTest static void d18_x_class_count() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)[a b]{1, 2}').matcher('ab'); Boolean f = m.find(); observation = 'find ' + f + ' end ' + (f ? String.valueOf(m.end()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d18_x_class_count2() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)[a b] x{2 ,3}').matcher('axxx'); Boolean f = m.find(); observation = 'find ' + f + ' end ' + (f ? String.valueOf(m.end()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d18_x_class_count3() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)[a b]a{1 0}').matcher('b' + 'a'.repeat(10)); Boolean f = m.find(); observation = 'find ' + f + ' end ' + (f ? String.valueOf(m.end()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d18_x_class_hex() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)[a b]\\x2 3').matcher('a#aa'); Boolean f = m.find(); observation = 'find ' + f + ' end ' + (f ? String.valueOf(m.end()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d18_x_class_oct() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)[a b]\\0 4 3').matcher('a#aa'); Boolean f = m.find(); observation = 'find ' + f + ' end ' + (f ? String.valueOf(m.end()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d18_x_class_uni() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)[a b]\\u0 0 2 3').matcher('a#aa'); Boolean f = m.find(); observation = 'find ' + f + ' end ' + (f ? String.valueOf(m.end()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d18_x_class_poss() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)[a b]a* +a').matcher('baaa'); Boolean f = m.find(); observation = 'find ' + f + ' end ' + (f ? String.valueOf(m.end()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d18_x_brace_hex() {
        String observation;
        try { Matcher m = Pattern.compile('(?x)\\x{4 1}').matcher('A'); Boolean f = m.find(); observation = 'find ' + f + ' end ' + (f ? String.valueOf(m.end()) : '-'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
