@IsTest
private class Rxd10762 {
    @IsTest static void d1_op_split_space() {
        String observation;
        try { observation = String.join('a b\tc\u00a0d'.split('\\p{Space}'),'|'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d1_op_replace_upper() {
        String observation;
        try { observation = 'aBcD'.replaceAll('\\p{Upper}','_'); } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
