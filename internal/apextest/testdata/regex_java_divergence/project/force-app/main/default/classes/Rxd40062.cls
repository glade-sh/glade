@IsTest
private class Rxd40062 {
    @IsTest static void d4_cr_caret() {
        String observation;
        try { observation = 'a\rb'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_cr_dollar() {
        String observation;
        try { observation = 'a\rb'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_cr_dot() {
        String observation;
        try { observation = 'a\rb'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_cr_plain_dollar() {
        String observation;
        try { observation = 'a\rb'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_cr_plain_caret() {
        String observation;
        try { observation = 'a\rb'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_cr_unix_caret() {
        String observation;
        try { observation = 'a\rb'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_cr_unix_dollar() {
        String observation;
        try { observation = 'a\rb'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_cr_dotall() {
        String observation;
        try { observation = 'a\rb'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_nel_caret() {
        String observation;
        try { observation = 'a\u0085b'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_nel_dollar() {
        String observation;
        try { observation = 'a\u0085b'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_nel_dot() {
        String observation;
        try { observation = 'a\u0085b'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_nel_plain_dollar() {
        String observation;
        try { observation = 'a\u0085b'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_nel_plain_caret() {
        String observation;
        try { observation = 'a\u0085b'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_nel_unix_caret() {
        String observation;
        try { observation = 'a\u0085b'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_nel_unix_dollar() {
        String observation;
        try { observation = 'a\u0085b'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_nel_dotall() {
        String observation;
        try { observation = 'a\u0085b'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ls_caret() {
        String observation;
        try { observation = 'a\u2028b'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ls_dollar() {
        String observation;
        try { observation = 'a\u2028b'.replaceAll('(?m)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ls_dot() {
        String observation;
        try { observation = 'a\u2028b'.replaceAll('(?m)^.*$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ls_plain_dollar() {
        String observation;
        try { observation = 'a\u2028b'.replaceAll('$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ls_plain_caret() {
        String observation;
        try { observation = 'a\u2028b'.replaceAll('^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ls_unix_caret() {
        String observation;
        try { observation = 'a\u2028b'.replaceAll('(?md)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ls_unix_dollar() {
        String observation;
        try { observation = 'a\u2028b'.replaceAll('(?md)$','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ls_dotall() {
        String observation;
        try { observation = 'a\u2028b'.replaceAll('(?s).','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d4_ps_caret() {
        String observation;
        try { observation = 'a\u2029b'.replaceAll('(?m)^','>').replace('\r','R').replace('\n','N').replace('\u0085','E').replace('\u2028','L').replace('\u2029','P'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
