@IsTest
private class Rxd10467 {
    @IsTest static void d1_p_empty() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Is_Alpha() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Is Alpha}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_javaLowercase_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{javaLowercase}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Alpha() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Alpha}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Alpha() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Alpha}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Lower() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Lower}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Lower() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Lower}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_Lower_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Lower}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Upper() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Upper}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Upper() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Upper}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_Upper_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Upper}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Alnum() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Alnum}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Alnum() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Alnum}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_Alnum_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Alnum}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Punct() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Punct}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Punct() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Punct}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_Punct_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Punct}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Digit() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Digit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Digit() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Digit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_Digit_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Digit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Space() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Space}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Space() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Space}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_Space_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Space}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Blank() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Blank}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Blank() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Blank}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
