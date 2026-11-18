@IsTest
private class Rxd30162 {
    @IsTest static void d3_str_first_one() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '$1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_one() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('$1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_one() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('$1'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_zero() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '$0'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_zero() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '$0'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_zero() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('$0'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_zero() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('$0'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_escdollar() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '\\$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_escdollar() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '\\$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_escdollar() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('\\$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_escdollar() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('\\$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_nope() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '${nope}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_nope() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '${nope}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_nope() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('${nope}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_nope() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('${nope}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_openbrace() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '${y'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_openbrace() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '${y'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_openbrace() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('${y'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_openbrace() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('${y'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_dollar2() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '$$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_dollar2() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '$$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_all_dollar2() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceAll('$$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_m_first_dollar2() {
        String observation;
        try { observation = Pattern.compile('(?<y>b)').matcher('abc').replaceFirst('$$'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_all_emptyname() {
        String observation;
        try { observation = 'abc'.replaceAll('(?<y>b)', '${}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d3_str_first_emptyname() {
        String observation;
        try { observation = 'abc'.replaceFirst('(?<y>b)', '${}'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
