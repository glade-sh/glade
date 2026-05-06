public class StdlibDatetimeProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'stdlib.datetime.valueOf-null') {
            return runValueOfNull();
        } else if (probeId == 'stdlib.datetime.leapYear') {
            return runLeapYear();
        } else if (probeId == 'stdlib.datetime.format-timezone') {
            return runFormatTimezone();
        } else if (probeId == 'stdlib.datetime.valueOf-invalid') {
            return runValueOfInvalid();
        } else if (probeId == 'stdlib.datetime.yearZero') {
            return runYearZero();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runValueOfNull() {
        // Edge case: DateTime.valueOf(null)
        String s = null;
        return DateTime.valueOf(s);
    }

    private Object runLeapYear() {
        // Edge case: Feb 29 on a leap year
        DateTime dt = DateTime.newInstance(2024, 2, 29, 0, 0, 0);
        return dt.format('yyyy-MM-dd');
    }

    private Object runFormatTimezone() {
        // Edge case: format with explicit timezone
        // Use newInstanceGmt so result is deterministic regardless of org user timezone
        DateTime dt = DateTime.newInstanceGmt(2024, 6, 15, 12, 0, 0);
        return dt.format('yyyy-MM-dd HH:mm:ss', 'America/Los_Angeles');
    }

    private Object runValueOfInvalid() {
        // Edge case: DateTime.valueOf with invalid string
        String s = 'not-a-datetime';
        return DateTime.valueOf(s);
    }

    private Object runYearZero() {
        // Edge case: year 0 boundary
        DateTime dt = DateTime.newInstance(0, 1, 1, 0, 0, 0);
        return dt.year();
    }
}
