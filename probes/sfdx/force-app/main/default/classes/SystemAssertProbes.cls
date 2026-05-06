public class SystemAssertProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'system.assert-pass') {
            return runAssertPass();
        } else if (probeId == 'system.assertEquals') {
            return runAssertEquals();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runAssertPass() {
        try {
            System.assert(true);
            return true;
        } catch (Exception e) {
            return false;
        }
    }

    private Object runAssertEquals() {
        try {
            System.assertEquals(1, 1);
            return true;
        } catch (Exception e) {
            return false;
        }
    }
}
