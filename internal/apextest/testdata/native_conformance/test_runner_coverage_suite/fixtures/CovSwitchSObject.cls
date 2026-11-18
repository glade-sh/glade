public class CovSwitchSObject {
    public static String kind(SObject rec) {
        String out = 'other';
        switch on rec {
            when Account a {
                out = 'account';
            }
            when Contact c {
                out = 'contact';
            }
            when else {
                out = 'other';
            }
        }
        return out;
    }
}
