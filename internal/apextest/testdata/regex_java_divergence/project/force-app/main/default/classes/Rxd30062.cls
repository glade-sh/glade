@IsTest
private class Rxd30062 {
    @IsTest static void d3_str_all_dollar() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_dollar() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_dollar() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_dollar() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_xdollar() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', 'x$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_xdollar() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', 'x$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_xdollar() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('x$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_xdollar() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('x$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_dollara() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '$a'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_dollara() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '$a'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_dollara() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('$a'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_dollara() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('$a'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_bslash() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '\\'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_bslash() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '\\'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_bslash() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('\\'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_bslash() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('\\'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_name() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '${y}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_name() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '${y}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_name() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('${y}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_name() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('${y}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_nine() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '$9'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_nine() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '$9'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_nine() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('$9'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_nine() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('$9'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_one() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '$1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
