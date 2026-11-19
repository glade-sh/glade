@IsTest
private class DecHashRound {
    @IsTest static void C05() {
        List<String> out = new List<String>();
        Decimal s3 = Decimal.valueOf('1.5').setScale(3, RoundingMode.HALF_UP);
        out.add('C05\tDecimal\t1.5\tsetScale_HALF_UP\t' + String.valueOf(s3) + '\t'
            + s3.precision() + '\t' + s3.scale() + '\t' + String.valueOf(s3.hashCode()) + '\t\t\t\t');
        out.add('END');
        System.assert(false, '\n' + String.join(out, '\n'));
    }
}
