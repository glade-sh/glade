@IsTest
private class Rxd140067 {
    @IsTest static void d14_aLQNbR() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!b)').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aLQEbR() {
        String observation;
        try { Matcher m = Pattern.compile('a(?=b)').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aLQTNbR() {
        String observation;
        try { Matcher m = Pattern.compile('a(?<!b)').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aLQTEbR() {
        String observation;
        try { Matcher m = Pattern.compile('a(?<=b)').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_a_b() {
        String observation;
        try { Matcher m = Pattern.compile('a\\b').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aD() {
        String observation;
        try { Matcher m = Pattern.compile('a$').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_a_z() {
        String observation;
        try { Matcher m = Pattern.compile('a\\z').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_a_Z_c() {
        String observation;
        try { Matcher m = Pattern.compile('a\\Z').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aLQNbRD() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!b)$').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aLQNbOcR() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!b|c)').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aLQNR() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!)').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_LQNbR() {
        String observation;
        try { Matcher m = Pattern.compile('(?!b)').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aSLQNbR() {
        String observation;
        try { Matcher m = Pattern.compile('a*(?!b)').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_abQLQNcR() {
        String observation;
        try { Matcher m = Pattern.compile('ab?(?!c)').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aLQEbRQ() {
        String observation;
        try { Matcher m = Pattern.compile('a(?=b)?').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_a_B_c() {
        String observation;
        try { Matcher m = Pattern.compile('a\\B').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_D() {
        String observation;
        try { Matcher m = Pattern.compile('$').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_C() {
        String observation;
        try { Matcher m = Pattern.compile('^').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_b() {
        String observation;
        try { Matcher m = Pattern.compile('\\b').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_aObD() {
        String observation;
        try { Matcher m = Pattern.compile('a|b$').matcher('a'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_neg_lookahead_nomatch() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!b)').matcher('ab'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d14_neg_lookahead_two() {
        String observation;
        try { Matcher m = Pattern.compile('a(?!b)').matcher('ac'); Boolean f = m.find(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
