@IsTest
private class Rxd40567 {
    @IsTest static void d4_matches_m_dollar_matches() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?m)a$\n?').matcher('a\n').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_split_lineanchor() {
        String observation;
        try { observation = String.join('a\rb\nc\r\nd\u0085e'.split('(?m)^'), '|').replace('\r','R').replace('\n','N').replace('\u0085','E'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_split_R() {
        String observation;
        try { observation = String.join('a\rb\nc\r\nd\u0085e\u2028f'.split('\\R'), '|'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
