public class MoreMathProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'math.abs-negative') {
            return runAbsNegative();
        } else if (probeId == 'math.max') {
            return runMax();
        } else if (probeId == 'math.pow') {
            return runPow();
        } else if (probeId == 'math.min') {
            return runMin();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runAbsNegative() {
        return Math.abs(-42);
    }

    private Object runMax() {
        return Math.max(3, 7);
    }

    private Object runPow() {
        return Math.pow(2, 3);
    }

    private Object runMin() {
        return Math.min(3, 7);
    }
}
