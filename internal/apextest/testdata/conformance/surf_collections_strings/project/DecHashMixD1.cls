@IsTest
private class DecHashMixD1 {
    @IsTest static void M01() {
        List<String> out = new List<String>();
        Double x = 0.1;
        Decimal d = x + 0.2;
        out.add('M01\tDecimal\t0.1+0.2\tdouble_plus_decimal\t' + String.valueOf(d) + '\t'
            + d.precision() + '\t' + d.scale() + '\t' + String.valueOf(d.hashCode()) + '\t\t\t\tx_is_Double');
        out.add('END');
        System.assert(false, '\n' + String.join(out, '\n'));
    }
}
