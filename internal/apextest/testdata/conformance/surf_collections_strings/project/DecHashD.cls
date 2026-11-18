@IsTest
private class DecHashD {
    @IsTest static void C01() {
        List<String> out = new List<String>();
        Double dv = 1.5d;
        out.add('C01\tDouble\t1.5d\td_suffix\t' + String.valueOf(dv) + '\t\t\t'
            + String.valueOf(dv.hashCode()) + '\t\t\t\td_suffix_compiles');
        out.add('END');
        System.assert(false, '\n' + String.join(out, '\n'));
    }
}
