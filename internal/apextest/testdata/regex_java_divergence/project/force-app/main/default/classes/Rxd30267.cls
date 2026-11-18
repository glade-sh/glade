@IsTest
private class Rxd30267 {
    @IsTest static void d3_m_all_emptyname() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('${}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_emptyname() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('${}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_num_brace() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '${1}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_num_brace() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '${1}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_num_brace() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('${1}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_num_brace() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('${1}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_esc_n() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '\\n'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_esc_n() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '\\n'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_esc_n() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('\\n'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_esc_n() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('\\n'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_esc_digit() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '\\1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_esc_digit() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '\\1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_esc_digit() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('\\1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_esc_digit() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('\\1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_minus() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '$-1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_minus() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '$-1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_minus() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('$-1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_minus() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('$-1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_twelve() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '$12'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_twelve() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '$12'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_twelve() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('$12'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_twelve() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('$12'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_empty() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', ''); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_empty() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', ''); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_empty() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll(''); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
