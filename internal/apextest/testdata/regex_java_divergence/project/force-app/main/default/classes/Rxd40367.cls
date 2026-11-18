@IsTest
private class Rxd40367 {
    @IsTest static void d4_trail_cr_plain_dollar() {
        String observation;
        try { observation = 'a\r'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_cr_plain_caret() {
        String observation;
        try { observation = 'a\r'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_cr_unix_caret() {
        String observation;
        try { observation = 'a\r'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_cr_unix_dollar() {
        String observation;
        try { observation = 'a\r'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_cr_dotall() {
        String observation;
        try { observation = 'a\r'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_crlf_caret() {
        String observation;
        try { observation = 'a\r\n'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_crlf_dollar() {
        String observation;
        try { observation = 'a\r\n'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_crlf_dot() {
        String observation;
        try { observation = 'a\r\n'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_crlf_plain_dollar() {
        String observation;
        try { observation = 'a\r\n'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_crlf_plain_caret() {
        String observation;
        try { observation = 'a\r\n'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_crlf_unix_caret() {
        String observation;
        try { observation = 'a\r\n'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_crlf_unix_dollar() {
        String observation;
        try { observation = 'a\r\n'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_crlf_dotall() {
        String observation;
        try { observation = 'a\r\n'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lead_cr_caret() {
        String observation;
        try { observation = '\ra'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lead_cr_dollar() {
        String observation;
        try { observation = '\ra'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lead_cr_dot() {
        String observation;
        try { observation = '\ra'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lead_cr_plain_dollar() {
        String observation;
        try { observation = '\ra'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lead_cr_plain_caret() {
        String observation;
        try { observation = '\ra'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lead_cr_unix_caret() {
        String observation;
        try { observation = '\ra'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lead_cr_unix_dollar() {
        String observation;
        try { observation = '\ra'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lead_cr_dotall() {
        String observation;
        try { observation = '\ra'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_empty_caret() {
        String observation;
        try { observation = ''.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_empty_dollar() {
        String observation;
        try { observation = ''.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_empty_dot() {
        String observation;
        try { observation = ''.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_empty_plain_dollar() {
        String observation;
        try { observation = ''.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
