@IsTest
private class Rxd20367 {
    @IsTest static void d2_replaceFirst_brace2() {
        String observation;
        try { observation = 'a'.replaceFirst('a{2,1}', 'x'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d2_matches_brace2() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('a{2,1}', 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d2_catchSE() {
        String observation;
        try { try { 'a'.split('['); observation='nothrow'; } catch (StringException e) { observation = 'SE ' + e.getMessage().replace('\n','\\n'); } } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d2_catchSE_compile() {
        String observation;
        try { try { Pattern.compile('['); observation='nothrow'; } catch (StringException e) { observation = 'SE'; } } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
