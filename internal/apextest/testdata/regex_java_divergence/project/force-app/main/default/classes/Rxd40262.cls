@IsTest
private class Rxd40262 {
    @IsTest static void d4_lf_dot() {
        String observation;
        try { observation = 'a\nb'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lf_plain_dollar() {
        String observation;
        try { observation = 'a\nb'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lf_plain_caret() {
        String observation;
        try { observation = 'a\nb'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lf_unix_caret() {
        String observation;
        try { observation = 'a\nb'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lf_unix_dollar() {
        String observation;
        try { observation = 'a\nb'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_lf_dotall() {
        String observation;
        try { observation = 'a\nb'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crcr_caret() {
        String observation;
        try { observation = 'a\r\rb'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crcr_dollar() {
        String observation;
        try { observation = 'a\r\rb'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crcr_dot() {
        String observation;
        try { observation = 'a\r\rb'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crcr_plain_dollar() {
        String observation;
        try { observation = 'a\r\rb'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crcr_plain_caret() {
        String observation;
        try { observation = 'a\r\rb'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crcr_unix_caret() {
        String observation;
        try { observation = 'a\r\rb'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crcr_unix_dollar() {
        String observation;
        try { observation = 'a\r\rb'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_crcr_dotall() {
        String observation;
        try { observation = 'a\r\rb'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_lf_caret() {
        String observation;
        try { observation = 'a\n'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_lf_dollar() {
        String observation;
        try { observation = 'a\n'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_lf_dot() {
        String observation;
        try { observation = 'a\n'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_lf_plain_dollar() {
        String observation;
        try { observation = 'a\n'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_lf_plain_caret() {
        String observation;
        try { observation = 'a\n'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_lf_unix_caret() {
        String observation;
        try { observation = 'a\n'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_lf_unix_dollar() {
        String observation;
        try { observation = 'a\n'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_lf_dotall() {
        String observation;
        try { observation = 'a\n'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_cr_caret() {
        String observation;
        try { observation = 'a\r'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_cr_dollar() {
        String observation;
        try { observation = 'a\r'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_trail_cr_dot() {
        String observation;
        try { observation = 'a\r'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
