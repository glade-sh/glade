@IsTest
private class DecHashDiv {
    @IsTest static void C02() {
        List<String> out = new List<String>();
        Decimal q = 5L / 2;
        out.add('C02\tDecimal\t5L/2\tlong_div\t' + String.valueOf(q) + '\t' + q.precision() + '\t'
            + q.scale() + '\t' + String.valueOf(q.hashCode()) + '\t\t\t\tlong_integer_division');
        Decimal q2 = 5L / 2.0;
        out.add('C03\tDecimal\t5L/2.0\tlong_div_decimal\t' + String.valueOf(q2) + '\t'
            + q2.precision() + '\t' + q2.scale() + '\t' + String.valueOf(q2.hashCode()) + '\t\t\t\t');
        out.add('END');
        System.assert(false, '\n' + String.join(out, '\n'));
    }
}
