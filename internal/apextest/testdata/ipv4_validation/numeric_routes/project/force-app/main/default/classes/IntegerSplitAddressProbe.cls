public class IntegerSplitAddressProbe {
    public static Boolean valid(String text) {
        List<String> pieces = text.split('\\.');
        if (pieces.size() != 4) { return false; }
        try {
            for (String piece : pieces) {
                Integer octet = Integer.valueOf(piece);
                if (octet < 0 || octet > 255) { return false; }
            }
        } catch (Exception error) { return false; }
        return true;
    }
}
