@IsTest
private class Rxd10167 {
    @IsTest static void d1_p_javaISOControl() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{javaISOControl}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_javaJavaIdentifierStart() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{javaJavaIdentifierStart}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_javaJavaIdentifierPart() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{javaJavaIdentifierPart}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_javaUnicodeIdentifierStart() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{javaUnicodeIdentifierStart}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsAlphabetic() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsAlphabetic}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsIdeographic() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsIdeographic}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsLetter() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsLetter}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsLowercase() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsLowercase}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsUppercase() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsUppercase}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsTitlecase() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsTitlecase}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsPunctuation() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsPunctuation}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsControl() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsControl}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsWhite_Space() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsWhite_Space}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsWhiteSpace() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsWhiteSpace}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsDigit() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsDigit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsHex_Digit() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsHex_Digit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsHexDigit() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsHexDigit}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsJoin_Control() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsJoin_Control}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsNoncharacter_Code_Point() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsNoncharacter_Code_Point}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsAssigned() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsAssigned}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsEmoji() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsEmoji}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsEmoji_Presentation() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsEmoji_Presentation}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsEmoji_Modifier() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsEmoji_Modifier}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsEmoji_Modifier_Base() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsEmoji_Modifier_Base}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_p_IsEmoji_Component() {
        String observation;
        try { String o=''; for (String c : new List<String>{'A','a','\u00e9','\u00c9','1','\u0660',' ','!','\u00a1','_','\u00a0','\u03b1','\u4e2d','\u0301','\uD83D\uDE00','\u01c5'}) { o += Pattern.matches('\\p{IsEmoji_Component}', c) ? '1' : '0'; } observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
