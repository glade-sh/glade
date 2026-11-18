@IsTest
private class Rxd200062 {
    @IsTest static void d20_name_greek() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{GREEK SMALL LETTER ALPHA}').matcher(String.fromCharArray(new List<Integer>{945})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_greek_lower() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{greek small letter alpha}').matcher(String.fromCharArray(new List<Integer>{945})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_underscore() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{LATIN_SMALL_LETTER_A}').matcher(String.fromCharArray(new List<Integer>{97})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_cjk_hyphen() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{CJK UNIFIED IDEOGRAPH-4E00}').matcher(String.fromCharArray(new List<Integer>{19968})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_hangul() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{HANGUL SYLLABLE GA}').matcher(String.fromCharArray(new List<Integer>{44032})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_cjk_block() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{CJK UNIFIED IDEOGRAPHS 4E00}').matcher(String.fromCharArray(new List<Integer>{19968})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_hangul_block() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{HANGUL SYLLABLES AC00}').matcher(String.fromCharArray(new List<Integer>{44032})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_trim() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{  LATIN SMALL LETTER A  }').matcher(String.fromCharArray(new List<Integer>{97})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_null() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{NULL}').matcher(String.fromCharArray(new List<Integer>{0})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_linefeed() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{LINE FEED}').matcher(String.fromCharArray(new List<Integer>{10})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_linefeed_lf() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{LINE FEED (LF)}').matcher(String.fromCharArray(new List<Integer>{10})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_block41() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{BASIC LATIN 41}').matcher(String.fromCharArray(new List<Integer>{65})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_block0041() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{BASIC LATIN 0041}').matcher(String.fromCharArray(new List<Integer>{65})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_dblspace() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{LATIN SMALL LETTER  A}').matcher(String.fromCharArray(new List<Integer>{97})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_hyphens() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{LATIN-SMALL-LETTER-A}').matcher(String.fromCharArray(new List<Integer>{97})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_nbsp() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{NO-BREAK SPACE}').matcher(String.fromCharArray(new List<Integer>{160})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_delete() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{DELETE}').matcher(String.fromCharArray(new List<Integer>{127})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_cjk_nohyphen() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{CJK UNIFIED IDEOGRAPH 4E00}').matcher(String.fromCharArray(new List<Integer>{19968})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d20_name_grave() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{LATIN SMALL LETTER A WITH GRAVE}').matcher(String.fromCharArray(new List<Integer>{224})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
