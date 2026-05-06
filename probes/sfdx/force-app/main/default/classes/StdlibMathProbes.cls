public class StdlibMathProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'stdlib.math.divide-scale') {
            return runDivideScale();
        } else if (probeId == 'stdlib.math.mod-negative') {
            return runModNegative();
        } else if (probeId == 'stdlib.math.round-halfUp') {
            return runRoundHalfUp();
        } else if (probeId == 'stdlib.math.decimalValueOf-null') {
            return runDecimalValueOfNull();
        } else if (probeId == 'stdlib.math.log10') {
            return runLog10();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runDivideScale() {
        Decimal d = Decimal.valueOf('1.00');
        return d.divide(3, 2, System.RoundingMode.HALF_UP);
    }

    private Object runModNegative() {
        return Math.mod(-5, 2);
    }

    private Object runRoundHalfUp() {
        return Math.round(2.5);
    }

    private Object runDecimalValueOfNull() {
        String s = null;
        return Decimal.valueOf(s);
    }

    private Object runLog10() {
        return Math.log10(100);
    }
}
