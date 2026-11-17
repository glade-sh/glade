@IsTest
private class Rxd14x0067 {
    @IsTest static void d14x_bz() {
        String observation;
        try { Matcher m = Pattern.compile('\\bz').matcher('ab'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_bw() {
        String observation;
        try { Matcher m = Pattern.compile('\\b\\w+').matcher('  '); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_dollar_nl_x() {
        String observation;
        try { Matcher m = Pattern.compile('b$x').matcher('ab\n'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_dollar_min() {
        String observation;
        try { Matcher m = Pattern.compile('b$x').matcher('ab'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_mdollar_nl_x() {
        String observation;
        try { Matcher m = Pattern.compile('(?m)b$x').matcher('ab\n'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_bigZ_nl_x() {
        String observation;
        try { Matcher m = Pattern.compile('b\\Zx').matcher('ab\n'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_neg_mid() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!b)x').matcher('ab'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_second_find() {
        String observation;
        try { Matcher m = Pattern.compile('a\\B').matcher('aXa'); m.find(); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_lookahead_dollar() {
        String observation;
        try { Matcher m = Pattern.compile('a(?=b$)c').matcher('ab'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_empty_end_refind() {
        String observation;
        try { Matcher m = Pattern.compile('$').matcher('a'); m.find(); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_third_find() {
        String observation;
        try { Matcher m = Pattern.compile('a\\B').matcher('aXa'); m.find(); m.find(); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_neg_body_dollar() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!b$)c').matcher('ab'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_neg_body_first() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!b|b$)').matcher('ab'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_neg_body_dollar_first() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!b$|b)').matcher('ab'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_usepattern_refind() {
        String observation;
        try { Matcher m = Pattern.compile('a\\B').matcher('a'); m.find(); m.usePattern(Pattern.compile('q')); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_usepattern_rematch() {
        String observation;
        try { Matcher m = Pattern.compile('a\\B').matcher('a'); m.find(); m.usePattern(Pattern.compile('a')); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14x_caret_third_find() {
        String observation;
        try { Matcher m = Pattern.compile('^').matcher('ab'); m.find(); m.find(); observation = String.valueOf(m.find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
