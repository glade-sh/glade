@IsTest
private class Rxd10362 {
    @IsTest static void d1_p_N() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{N}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Nd() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Nd}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Nl() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Nl}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_No() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{No}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_P() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{P}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Pd() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Pd}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_S() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{S}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Sc() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Sc}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Sm() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Sm}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Z() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Z}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Zs() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Zs}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_C() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{C}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Cc() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Cc}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Cf() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Cf}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Cn() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Cn}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Co() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Co}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_LC() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{LC}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsL() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsL}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsLu() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsLu}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsLetter_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsLetter}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Foo() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Foo}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsFoo() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsFoo}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_InFoo() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{InFoo}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_script_Foo() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{script=Foo}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Alpha_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{^Alpha}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
