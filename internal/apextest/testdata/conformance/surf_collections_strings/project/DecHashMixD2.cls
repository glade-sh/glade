@IsTest
private class DecHashMixD2 {
    @IsTest static void M02() {
        List<String> out = new List<String>();
        Decimal d4 = 0.1d + 0.2d;
        out.add('M02\tDecimal\t0.1d+0.2d\tdouble_literal_sum\t' + String.valueOf(d4) + '\t'
            + d4.precision() + '\t' + d4.scale() + '\t' + String.valueOf(d4.hashCode()) + '\t\t\t\t');
        out.add('END');
        System.assert(false, '\n' + String.join(out, '\n'));
    }
}
