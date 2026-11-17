@IsTest
private class DecHashWrap2 {
    @IsTest static void C07() {
        List<String> out = new List<String>();
        out.add('C07\tDouble\t1.5\twrap_double\t' + String.valueOf(Double.valueOf('1.5')) + '\t\t\t'
            + String.valueOf(Double.valueOf('1.5').hashCode()) + '\t\t\t\tDouble.valueOf');
        out.add('C07\tInteger\t1\twrap_integer\t' + String.valueOf(Integer.valueOf(1)) + '\t\t\t'
            + String.valueOf(Integer.valueOf(1).hashCode()) + '\t\t\t\tInteger.valueOf');
        out.add('C07\tDecimal\t1.5\twrap_decimal\t' + String.valueOf(Decimal.valueOf('1.5')) + '\t\t\t'
            + String.valueOf(Decimal.valueOf('1.5').hashCode()) + '\t\t\t\tDecimal.valueOf');
        out.add('END');
        System.assert(false, '\n' + String.join(out, '\n'));
    }
}
