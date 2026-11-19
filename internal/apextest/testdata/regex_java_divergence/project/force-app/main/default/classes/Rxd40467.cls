@IsTest
private class Rxd40467 {
    @IsTest static void d4_empty_plain_caret() {
        String observation;
        try { observation = ''.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_empty_unix_caret() {
        String observation;
        try { observation = ''.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_empty_unix_dollar() {
        String observation;
        try { observation = ''.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_empty_dotall() {
        String observation;
        try { observation = ''.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlfcrlf_caret() {
        String observation;
        try { observation = 'a\r\n\r\nb'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlfcrlf_dollar() {
        String observation;
        try { observation = 'a\r\n\r\nb'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlfcrlf_dot() {
        String observation;
        try { observation = 'a\r\n\r\nb'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlfcrlf_plain_dollar() {
        String observation;
        try { observation = 'a\r\n\r\nb'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlfcrlf_plain_caret() {
        String observation;
        try { observation = 'a\r\n\r\nb'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlfcrlf_unix_caret() {
        String observation;
        try { observation = 'a\r\n\r\nb'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlfcrlf_unix_dollar() {
        String observation;
        try { observation = 'a\r\n\r\nb'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlfcrlf_dotall() {
        String observation;
        try { observation = 'a\r\n\r\nb'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_dot_cr() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('.').matcher('\r').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_dot_nel() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('.').matcher('\u0085').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_dot_ls() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('.').matcher('\u2028').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_dot_ps() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('.').matcher('\u2029').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_dot_vt() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('.').matcher('\u000b').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_dot_ff() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('.').matcher('\u000c').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_d_dot_cr() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?d).').matcher('\r').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_d_dot_nel() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?d).').matcher('\u0085').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_d_dot_lf() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?d).').matcher('\n').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_dollar_before_crlf() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a$').matcher('a\r\n').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_dollar_before_nel() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a$').matcher('a\u0085').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_Z_before_nel() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a\\Z').matcher('a\u0085').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_matches_Z_before_crlf() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a\\Z').matcher('a\r\n').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
