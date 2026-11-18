@IsTest
private class Rxd100062 {
    @IsTest static void d10_m_esc_pair() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\uD83D\\uDE00', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_esc_pair_class() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\\uD83D\\uDE00]', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_esc_hi_only() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\uD83D', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_lit_pair() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\uD83D\uDE00', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_lit_class() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\uD83D\uDE00]', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_emoji_strip_range() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\\uD83C-\\uDBFF\\uDC00-\\uDFFF]+', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_emoji_strip_lit() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\uD83C-\uDBFF\uDC00-\uDFFF]+', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_dot() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('.', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_dot_dot() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('..', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_x_brace() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\x{1F600}', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_hi_range_class() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\\uD800-\\uDBFF]', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_lo_range_class() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\\uDC00-\\uDFFF]', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_lone_hi() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\\uD83D]', '\uD83D')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_lone_lo() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\uDE00', '\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_prop_So() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\p{So}', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_prop_Cs() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\p{Cs}', '\uD83D')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_range_astral() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\\x{1F600}-\\x{1F64F}]', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_neg_class() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[^a]', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_neg_class_dot2() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('^[^a]$', '\uD83D\uDE00')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_split_empty() {
        String observation;
        try { observation = String.valueOf('\uD83D\uDE00'.split('').size()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_split_empty_list() {
        String observation;
        try { observation = String.valueOf('\uD83D\uDE00'.split('').size()) + '|' + String.join('a\uD83D\uDE00b'.split(''), ',').length(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_find_count() {
        String observation;
        try { Matcher m = Pattern.compile('').matcher('\uD83D\uDE00'); Integer n=0; while(m.find()){n++;} observation = String.valueOf(n); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_find_starts() {
        String observation;
        try { Matcher m = Pattern.compile('').matcher('a'+'\uD83D\uDE00'+'b'); String o=''; while(m.find()){o += m.start()+',';} observation = o; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_replace_empty() {
        String observation;
        try { observation = String.valueOf('a\uD83D\uDE00b'.replaceAll('','-').length()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_replace_empty_vis() {
        String observation;
        try { observation = 'a\uD83D\uDE00b'.replaceAll('','-').replace('\uD83D','H').replace('\uDE00','L'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_lit_lone_hi() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\uD83D', '\uD83D')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_matcher_lone_hi() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\uD83D').matcher('\uD83D').matches()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_matcher_lit_lone_hi() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\uD83D').matcher('\uD83D').matches()); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_matcher_group_lone_hi() {
        String observation;
        try { Matcher m = Pattern.compile('(.)').matcher('\uD83D'); Boolean f = m.matches(); observation = f + ' ' + m.end(1); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_lookingat_lone_dollar() {
        String observation;
        try { Matcher m = Pattern.compile('.$').matcher('\uD83D'); Boolean f = m.lookingAt(); observation = f+' hitEnd='+m.hitEnd()+' requireEnd='+m.requireEnd(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d10_m_backslash_lone_hi() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\\uD83D', '\uD83D')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
