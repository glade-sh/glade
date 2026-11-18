@IsTest
private class Rxd50067 {
    @IsTest static void d5_e_acute() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\u00e9', '\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_kelvin() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)k', '\u212a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_kelvin() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)k', '\u212a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_long_s() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)s', '\u017f')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_long_s() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)s', '\u017f')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_range_latin1() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)[\u00e0-\u00ff]', '\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_sigma() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\u03c3', '\u03a3')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_sigma() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u03c3', '\u03a3')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_final_sigma() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\u03c2', '\u03a3')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_final_sigma() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u03c2', '\u03c3')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_final_sigma2() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u03c2', '\u03a3')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_sharp_s() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\u00df', '\u1e9e')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_sharp_s() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u00df', '\u1e9e')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_dz_title() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u01c6', '\u01c5')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_dz_upper() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u01c6', '\u01c4')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_dz_title() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\u01c6', '\u01c5')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_dotted_I() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u0130', 'i')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_dotted_I() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\u0130', 'i')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_dotless_i() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u0131', 'I')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_i_dotted_I() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)i', '\u0130')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_class_az_kelvin() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)[a-z]', '\u212a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_class_az_kelvin() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)[a-z]', '\u212a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_e_acute() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u00e9', '\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_ascii() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)a', 'A')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_greek_alpha() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\u03b1', '\u0391')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
