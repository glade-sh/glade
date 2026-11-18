@IsTest
private class Rxd220062 {
    @IsTest static void d22_name_bags() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{FACE WITH BAGS UNDER EYES}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d22_name_bell_bel() {
        String observation;
        try { Matcher m = Pattern.compile('\\N{BELL}').matcher(String.fromCharArray(new List<Integer>{7})); observation = 'find ' + m.find(); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d22_name_shaking() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{SHAKING FACE}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
