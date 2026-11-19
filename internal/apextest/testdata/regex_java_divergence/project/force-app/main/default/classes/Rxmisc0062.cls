@IsTest
private class Rxmisc0062 {
    @IsTest static void m_pat_debug() {
        String observation;
        try { Pattern p = Pattern.compile('a+'); observation = String.valueOf(p); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_pat_concat() {
        String observation;
        try { Pattern p = Pattern.compile('a+'); observation = '' + p; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_pat_tostring() {
        String observation;
        try { Pattern p = Pattern.compile('a+'); observation = p.toString(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_pat_pattern() {
        String observation;
        try { Pattern p = Pattern.compile('a+'); observation = p.pattern(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_m_valueof() {
        String observation;
        try { Matcher m = Pattern.compile('a+').matcher('aa'); observation = String.valueOf(m); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_m_tostring() {
        String observation;
        try { Matcher m = Pattern.compile('a+').matcher('aa'); observation = m.toString(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_m_after_find() {
        String observation;
        try { Matcher m = Pattern.compile('a+').matcher('aa'); m.find(); observation = m.toString(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_json_pattern() {
        String observation;
        try { Pattern p = Pattern.compile('a+'); observation = JSON.serialize(p.pattern()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_pat_equals() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a+') == Pattern.compile('a+')) + '|' + String.valueOf(Pattern.compile('a+').equals(Pattern.compile('a+'))); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_u1_a_plus() {
        String observation;
        try { observation = 'aabar'.replaceAll('(?<=a+)bar','Y'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_u1_a_brace() {
        String observation;
        try { observation = 'aabar'.replaceAll('(?<=a{1,})bar','Y'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_u2_b_replace() {
        String observation;
        try { observation = 'caf\u00e9 au'.replaceAll('\\b','|'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_u2_b_matches() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\b\u00e9\\b','\u00e9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_u2_b_find() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\b').matcher('\u00e9').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_u2_B() {
        String observation;
        try { observation = 'caf\u00e9'.replaceAll('\\B','|'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_u2_w() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\w','\u00e9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_u2_U_b() {
        String observation;
        try { observation = 'caf\u00e9 au'.replaceAll('(?U)\\b','|'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void m_u2_digit_b() {
        String observation;
        try { observation = '\u0660a'.replaceAll('\\b','|'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
