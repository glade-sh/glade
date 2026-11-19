@IsTest
private class Rxd8x0062 {
    @IsTest static void d8x_qplus_alt_q() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\Q+\\E|?')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_emoji_alt_star() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('😀|*')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_emoji_class() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('😀[')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_i_star() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?i)*')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_Qa_brace() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\Qa\\E{')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_x2_brace() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('x{2}{')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_class_emoji_open() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[😀')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_emoji_bs_y() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('😀\\y')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_QE_star() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\Q\\E*')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_x_alt_star() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?x) a |*')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_alt_rparen() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a|)')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_open_paren_ab() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(a|b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_open_nc() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?:a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_brace_open_comma() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a{1,2')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_brace_rev() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a{2,1}')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_p_latin_open() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\p{IsLatin')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_class_inter_open() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[a-z&&[def')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_k_open() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\k<n')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_k_unclosed_name() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<n>a)\\k<n')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_poss_all() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a++b*+c?+d{1}+')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_poss_brace() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('a{1}+')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_flag_off_on() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?i)(?-i)a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_range_aa() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[a-a]')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_range_w_z() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[\\w-z]')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_range_z_w() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[z-\\w]')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
