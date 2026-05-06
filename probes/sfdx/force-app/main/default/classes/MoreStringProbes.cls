public class MoreStringProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'string.escapeSingleQuotes') {
            return runEscapeSingleQuotes();
        } else if (probeId == 'string.split') {
            return runSplit();
        } else if (probeId == 'string.trim') {
            return runTrim();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runEscapeSingleQuotes() {
        return String.escapeSingleQuotes(String.fromCharArray(new List<Integer>{105, 116, 39, 115}));
    }

    private Object runSplit() {
        List<String> parts = 'a,b,c'.split(',');
        return parts.size();
    }

    private Object runTrim() {
        return '  hello  '.trim();
    }
}
