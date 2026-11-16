@IsTest
private class Rxd8x0167 {
    @IsTest static void d8x_lb_ab_plus() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=ab+)c')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_nc_ab_plus() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=(?:ab)+)c')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_grp_plus() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=(a)+)b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_brace_range() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=a{2,5})b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_alt_rep2() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=(?:a|b){2})c')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_alt_plus_b() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=a|b+)c')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_look_star() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?=a)*')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_neglook_plus() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?!a)+')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_caret_star() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('^*')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_dollar_plus() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('$+')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_b_star() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\b*')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_A_q() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('\\A?')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_ref() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(a)(?<=\\1)b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_lazy() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=a+?)b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_poss() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=a++)b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_nested_alt() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=(?:a|bc)d)e')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_big() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=a{1,2147483647})b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lb_group_star_fixed() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?<=(?:ab)*)c')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_x_comment_paren() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('(?x)(a # c )\n)')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_lone_amp() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[a&b]')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_amp_amp_only() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[&&]')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_neg_amp() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[^&&a]')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_nested_empty() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[[]]')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_nested_neg_empty() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[^]]')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d8x_class_hyphen_range_esc() {
        String observation;
        try { observation = String.valueOf(Pattern.compile('[a-\\]]')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
