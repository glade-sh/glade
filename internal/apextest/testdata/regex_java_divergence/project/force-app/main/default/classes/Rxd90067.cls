@IsTest
private class Rxd90067 {
    @IsTest static void d9_x_Q() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)\\Qa b\\E', 'a b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_Q_nospace() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)\\Qa b\\E', 'ab')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_space() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)a b', 'ab')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_stray_E() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('a\\Eb', 'ab')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_Q_open() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\Qa', 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_Q_class() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\\Qa-z\\E]', '-')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_Q_class_b() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\\Qa-z\\E]', 'b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_Q_class_a() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('[\\Qa-z\\E]', 'a')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_Q_empty() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\Q\\E', '')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_Q_dot() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\Qa.b\\E', 'a.b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_Q_dot2() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\Qa.b\\E', 'axb')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_Q_E_in_Q() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('\\Qa\\Eb\\E', 'ab')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_sp() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[a b]', ' ')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_b() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[a b]', 'b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_hash() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[a#b]', '#')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_hash_b() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[a#b]', 'b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_hash_comment() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)a#c\nb', 'ab')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_escaped_space() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)a\\ b', 'a b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_range_sp() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[a - c]', 'b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_range_sp_dash() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[a - c]', '-')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_range_sp_sp() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[a - c]', ' ')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_tab() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[a\tb]', '\t')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_nl() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[a\nb]', '\n')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_class_inner_sp() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)[\\p{L} ]', ' ')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d9_x_Q_hash() {
        String observation;
        try { observation = String.valueOf(Pattern.matches('(?x)\\Qa#b\\E', 'a#b')); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
