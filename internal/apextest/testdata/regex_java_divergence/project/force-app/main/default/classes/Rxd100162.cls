@IsTest
private class Rxd100162 {
    @IsTest static void d10_replace_dot() {
        String observation;
        try { observation = '\uD83D\uDE00'.replaceAll('.','x'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_replace_dot_count() {
        String observation;
        try { observation = String.valueOf('\uD83D\uDE00'.replaceAll('.','x').length()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_replace_star() {
        String observation;
        try { observation = String.valueOf('\uD83D\uDE00'.replaceAll('x*','-').length()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_split_x() {
        String observation;
        try { observation = String.valueOf('\uD83D\uDE00x\uD83D\uDE00'.split('x').size()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_lone_hi_find() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\uD83D').matcher('\uD83D\uDE00').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_lone_lo_find() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\uDE00').matcher('\uD83D\uDE00').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_lone_hi_lit_find() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\uD83D').matcher('\uD83D\uDE00').find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_strip_replace() {
        String observation;
        try { observation = String.valueOf('a\uD83D\uDE00b'.replaceAll('[\\uD83C-\\uDBFF\\uDC00-\\uDFFF]+','').length()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_strip_replace2() {
        String observation;
        try { observation = String.valueOf('a\uD83D\uDE00b'.replaceAll('[^\\u0000-\\uFFFF]','').length()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_strip_replace3() {
        String observation;
        try { observation = String.valueOf('a\uD83D\uDE00b'.replaceAll('[\\p{So}]','').length()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_region_astral() {
        String observation;
        try { Matcher m = Pattern.compile('.').matcher('\uD83D\uDE00'); m.region(1,2); observation = String.valueOf(m.find()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_start_end() {
        String observation;
        try { Matcher m = Pattern.compile('.').matcher('a\uD83D\uDE00'); m.find(); m.find(); observation = m.start()+','+m.end(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_length_class() {
        String observation;
        try { observation = String.valueOf('\uD83D\uDE00'.replaceAll('(?s).{2}','x').length()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
