@IsTest
private class Rxd30367 {
    @IsTest static void d3_m_first_empty() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst(''); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_replaceAll_regex() {
        String observation;
        try { String s = null; observation = 'abc'.replaceAll(s, 'x'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_replaceAll_repl() {
        String observation;
        try { String s = null; observation = 'abc'.replaceAll('b', s); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_replaceFirst_regex() {
        String observation;
        try { String s = null; observation = 'abc'.replaceFirst(s, 'x'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_replaceFirst_repl() {
        String observation;
        try { String s = null; observation = 'abc'.replaceFirst('b', s); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_split() {
        String observation;
        try { String s = null; observation = String.valueOf('abc'.split(s)); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_split2() {
        String observation;
        try { String s = null; observation = String.valueOf('abc'.split(s, 2)); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_compile() {
        String observation;
        try { String s = null; observation = String.valueOf(Pattern.compile(s)); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_pmatches_regex() {
        String observation;
        try { String s = null; observation = String.valueOf(Pattern.matches(s, 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_pmatches_input() {
        String observation;
        try { String s = null; observation = String.valueOf(Pattern.matches('a', s)); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_matcher() {
        String observation;
        try { String s = null; observation = String.valueOf(Pattern.compile('a').matcher(s)); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_m_replaceAll() {
        String observation;
        try { String s = null; observation = Pattern.compile('b').matcher('abc').replaceAll(s); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_m_replaceFirst() {
        String observation;
        try { String s = null; observation = Pattern.compile('b').matcher('abc').replaceFirst(s); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_usePattern() {
        String observation;
        try { Pattern p = null; observation = String.valueOf(Pattern.compile('b').matcher('abc').usePattern(p)); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_psplit() {
        String observation;
        try { String s = null; observation = String.valueOf(Pattern.compile('b').split(s)); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_n_input_replaceAll() {
        String observation;
        try { String s = null; observation = s.replaceAll('b','x'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_u5_str_all() {
        String observation;
        try { observation = 'abc'.replaceAll('(b)','$3'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_u5_str_first() {
        String observation;
        try { observation = 'abc'.replaceFirst('(b)','$3'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_u5_m_all() {
        String observation;
        try { observation = Pattern.compile('(b)').matcher('abc').replaceAll('$3'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_u5_m_first() {
        String observation;
        try { observation = Pattern.compile('(b)').matcher('abc').replaceFirst('$3'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_u5_nogroup() {
        String observation;
        try { observation = 'abc'.replaceAll('b','$1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
