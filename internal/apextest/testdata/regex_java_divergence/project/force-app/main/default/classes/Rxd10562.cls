@IsTest
private class Rxd10562 {
    @IsTest static void d1_P_Blank_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Blank}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Cntrl() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Cntrl}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Cntrl() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Cntrl}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_Cntrl_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Cntrl}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_XDigit() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{XDigit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_XDigit() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{XDigit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_XDigit_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{XDigit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Graph() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Graph}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Graph() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Graph}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_Graph_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Graph}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_Print() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{Print}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_Print() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{Print}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_Print_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{Print}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_ASCII() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{ASCII}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_ASCII() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{ASCII}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_ASCII_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{ASCII}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_IsAlphabetic() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{IsAlphabetic}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_IsAlphabetic() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{IsAlphabetic}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_IsAlphabetic_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{IsAlphabetic}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_IsDigit() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{IsDigit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_IsDigit() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{IsDigit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_IsDigit_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{IsDigit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_U_IsUppercase() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?U)\\p{IsUppercase}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_i_IsUppercase() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('(?i)\\p{IsUppercase}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_P_IsUppercase_c() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\P{IsUppercase}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
