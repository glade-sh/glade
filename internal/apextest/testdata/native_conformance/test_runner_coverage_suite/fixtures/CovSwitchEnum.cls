public class CovSwitchEnum {
    public enum Phase { OPEN, HOLD, CLOSED }

    public static String describe(Phase p) {
        String out = 'closed';
        switch on p {
            when OPEN {
                out = 'open';
            }
            when HOLD {
                out = 'hold';
            }
            when else {
                out = 'closed';
            }
        }
        return out;
    }
}
