@IsTest
private class Rxd10267 {
    @IsTest static void d1_p_IsExtended_Pictographic() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsExtended_Pictographic}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_InGreek() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{InGreek}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_InBasicLatin() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{InBasicLatin}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_InLatin_1Supplement() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{InLatin-1Supplement}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_InGreek_and_Coptic() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{InGreek_and_Coptic}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_InCJKUnifiedIdeographs() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{InCJKUnifiedIdeographs}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsLatin() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsLatin}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsGreek() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsGreek}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsHan() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsHan}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsCommon() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsCommon}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Latin() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Latin}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_script_Greek() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{script=Greek}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_sc_Greek() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{sc=Greek}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_block_Greek() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{block=Greek}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_blk_BasicLatin() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{blk=BasicLatin}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_general_category_Lu() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{general_category=Lu}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_gc_Lu() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{gc=Lu}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_L() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{L}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Lu() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Lu}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Ll() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Ll}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Lt() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Lt}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Lm() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Lm}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Lo() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Lo}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_M() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{M}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_Mn() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{Mn}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
