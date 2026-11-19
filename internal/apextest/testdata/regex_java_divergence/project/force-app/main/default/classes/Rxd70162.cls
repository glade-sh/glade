@IsTest
private class Rxd70162 {
    @IsTest static void d7_18_nogroup() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\18', '\u00018')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_1_nogroup() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\1', '\u0001')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_x41() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\x41', 'A')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_x_brace() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\x{41}', 'A')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_x_short() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\x4', 'A')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_x_none() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\x', 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_u0041() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\u0041', 'A')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_u_short() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\u004', 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_cA() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\cA', '\u0001')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_c_none() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\c', 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_x_big() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\x{110000}', 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_x_brace_astral() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\x{1F600}', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d7_x_brace_open() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\x{41', 'A')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
