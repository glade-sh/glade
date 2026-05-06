public class StdlibStringProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'stdlib.string.format-null') {
            return runFormatNull();
        } else if (probeId == 'stdlib.string.join-empty') {
            return runJoinEmpty();
        } else if (probeId == 'stdlib.string.containsIgnoreCase-null') {
            return runContainsIgnoreCaseNull();
        } else if (probeId == 'stdlib.string.valueOf-null') {
            return runValueOfNull();
        } else if (probeId == 'stdlib.string.isBlank-whitespace') {
            return runIsBlankWhitespace();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runFormatNull() {
        List<String> args = new List<String>();
        args.add(null);
        return String.format('Hello {0}', args);
    }

    private Object runJoinEmpty() {
        List<String> items = new List<String>();
        return String.join(items, ',');
    }

    private Object runContainsIgnoreCaseNull() {
        String s = 'abc';
        return s.containsIgnoreCase(null);
    }

    private Object runValueOfNull() {
        Object o = null;
        return String.valueOf(o);
    }

    private Object runIsBlankWhitespace() {
        String s = '  \n\t  ';
        return 'isBlank=' + String.isBlank(s) + ',isEmpty=' + String.isEmpty(s);
    }
}
