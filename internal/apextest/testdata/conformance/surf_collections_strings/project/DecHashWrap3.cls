@IsTest
private class DecHashWrap3 {
    @IsTest static void C08() {
        List<String> out = new List<String>();
        Long lv = 1L;
        out.add('C08\tLong\t1\tlocal_long\t' + String.valueOf(lv) + '\t\t\t'
            + String.valueOf(lv.hashCode()) + '\t\t\t\tlocal_variable');
        Integer iv = 1;
        out.add('C08\tInteger\t1\tlocal_integer\t' + String.valueOf(iv) + '\t\t\t'
            + String.valueOf(iv.hashCode()) + '\t\t\t\tlocal_variable');
        out.add('END');
        System.assert(false, '\n' + String.join(out, '\n'));
    }
}
