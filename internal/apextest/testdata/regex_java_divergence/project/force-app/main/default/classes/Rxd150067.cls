@IsTest
private class Rxd150067 {
    @IsTest static void d15_hitend_anch_24() {
        String observation;
        try { Matcher m = Pattern.compile('^(a+)+b').matcher('a'.repeat(24)); Boolean f = m.find(); observation = 'find ' + f + ' hitEnd ' + m.hitEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d15_hitend_anch_23c() {
        String observation;
        try { Matcher m = Pattern.compile('^(a+)+b').matcher('a'.repeat(23) + 'c'); Boolean f = m.find(); observation = 'find ' + f + ' hitEnd ' + m.hitEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d15_hitend_unanch_23c() {
        String observation;
        try { Matcher m = Pattern.compile('(a+)+b').matcher('a'.repeat(23) + 'c'); Boolean f = m.find(); observation = 'find ' + f + ' hitEnd ' + m.hitEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d15_hitend_group_23c() {
        String observation;
        try { Matcher m = Pattern.compile('(^(a+)+b)').matcher('a'.repeat(23) + 'c'); Boolean f = m.find(); observation = 'find ' + f + ' hitEnd ' + m.hitEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d15_hitend_ncgroup_23c() {
        String observation;
        try { Matcher m = Pattern.compile('(?:^(a+)+b)').matcher('a'.repeat(23) + 'c'); Boolean f = m.find(); observation = 'find ' + f + ' hitEnd ' + m.hitEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d15_fold_longs() {
        String observation;
        try { Matcher m = Pattern.compile('(?i)^(s+)+x').matcher('s'.repeat(20) + '\u017F'); Boolean f = m.find(); observation = 'find ' + f + ' hitEnd ' + m.hitEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
