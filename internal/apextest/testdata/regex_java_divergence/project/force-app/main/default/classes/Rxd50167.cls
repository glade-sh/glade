@IsTest
private class Rxd50167 {
    @IsTest static void d5_cyrillic() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\u0430', '\u0410')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_cyrillic() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)\u0430', '\u0410')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_backref() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)(\u00e9)\\1', '\u00e9\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_backref() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)(\u00e9)\\1', '\u00e9\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_prop_lu() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\\p{Lu}', 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_prop_ll() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\\p{Ll}', 'A')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_upper_posix() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\\p{Upper}', 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_lower_posix() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)\\p{Lower}', 'A')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_neg_class() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)[^a]', 'A')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_neg_class_e() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu)[^\u00e9]', '\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_flag_off() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i)(?-i)\u00e9', '\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_scoped() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?i:\u00e9)', '\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_u_scoped() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?iu:\u00e9)', '\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_inline_u_then_i() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?u)(?i)\u00e9', '\u00c9')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d5_combined() {
        String observation;
        try { observation = Pattern.matches('(?i)\u00e9','\u00c9') + '|' + Pattern.matches('(?i)k','\u212a') + '|' + Pattern.matches('(?iu)k','\u212a') + '|' + Pattern.matches('(?iu)s','\u017f') + '|' + Pattern.matches('(?i)[\u00e0-\u00ff]','\u00c9') + '|' + Pattern.matches('(?i)\u03c3','\u03a3'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
