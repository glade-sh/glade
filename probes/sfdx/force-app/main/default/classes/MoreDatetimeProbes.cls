public class MoreDatetimeProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'datetime.now') {
            return runNow();
        } else if (probeId == 'datetime.today') {
            return runToday();
        } else if (probeId == 'datetime.valueOfGmt') {
            return runValueOfGmt();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runNow() {
        DateTime dt = DateTime.now();
        return dt != null;
    }

    private Object runToday() {
        Date d = Date.today();
        return d != null;
    }

    private Object runValueOfGmt() {
        DateTime dt = DateTime.valueOfGmt('2024-06-15 12:00:00');
        return dt.formatGmt('yyyy-MM-dd HH:mm:ss');
    }
}
