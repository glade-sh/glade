public class GroupedAddressProbe {
    public static Boolean valid(String text) {
        Matcher match = Pattern.compile('^([0-9]+)\\.([0-9]+)\\.([0-9]+)\\.([0-9]+)$').matcher(text);
        if (!match.matches()) { return false; }
        try {
            for (Integer groupIndex=1; groupIndex<=4; groupIndex++) {
                Integer octet = Integer.valueOf(match.group(groupIndex));
                if (octet < 0 || octet > 255) { return false; }
            }
        } catch (Exception error) { return false; }
        return true;
    }
}
