@IsTest
private class Rxd8x0267 {
    @IsTest static void d8x_hex_brace_empty() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\x{}')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_u_bs() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\u{41}')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_cX_hi() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\cé')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
