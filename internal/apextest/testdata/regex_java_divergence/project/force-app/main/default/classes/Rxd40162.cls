@IsTest
private class Rxd40162 {
    @IsTest static void d4_ps_dollar() {
        String observation;
        try { observation = 'a\u2029b'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ps_dot() {
        String observation;
        try { observation = 'a\u2029b'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ps_plain_dollar() {
        String observation;
        try { observation = 'a\u2029b'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ps_plain_caret() {
        String observation;
        try { observation = 'a\u2029b'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ps_unix_caret() {
        String observation;
        try { observation = 'a\u2029b'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ps_unix_dollar() {
        String observation;
        try { observation = 'a\u2029b'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ps_dotall() {
        String observation;
        try { observation = 'a\u2029b'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlf_caret() {
        String observation;
        try { observation = 'a\r\nb'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlf_dollar() {
        String observation;
        try { observation = 'a\r\nb'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlf_dot() {
        String observation;
        try { observation = 'a\r\nb'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlf_plain_dollar() {
        String observation;
        try { observation = 'a\r\nb'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlf_plain_caret() {
        String observation;
        try { observation = 'a\r\nb'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlf_unix_caret() {
        String observation;
        try { observation = 'a\r\nb'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlf_unix_dollar() {
        String observation;
        try { observation = 'a\r\nb'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crlf_dotall() {
        String observation;
        try { observation = 'a\r\nb'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lfcr_caret() {
        String observation;
        try { observation = 'a\n\rb'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lfcr_dollar() {
        String observation;
        try { observation = 'a\n\rb'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lfcr_dot() {
        String observation;
        try { observation = 'a\n\rb'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lfcr_plain_dollar() {
        String observation;
        try { observation = 'a\n\rb'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lfcr_plain_caret() {
        String observation;
        try { observation = 'a\n\rb'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lfcr_unix_caret() {
        String observation;
        try { observation = 'a\n\rb'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lfcr_unix_dollar() {
        String observation;
        try { observation = 'a\n\rb'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lfcr_dotall() {
        String observation;
        try { observation = 'a\n\rb'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lf_caret() {
        String observation;
        try { observation = 'a\nb'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lf_dollar() {
        String observation;
        try { observation = 'a\nb'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
