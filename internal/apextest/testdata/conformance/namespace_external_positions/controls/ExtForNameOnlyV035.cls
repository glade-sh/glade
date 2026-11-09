public class ExtForNameOnlyV035 {
    public static String observe() {
        Type dotted = Type.forName('MissingNsV035.Thing');
        Type twoArg = Type.forName('MissingNsV035', 'Thing');
        String constructed;
        try { Object o = dotted.newInstance(); constructed = 'constructed'; } catch (Exception e) { constructed = e.getTypeName(); }
        return (dotted == null) + '|' + (twoArg == null) + '|' + constructed;
    }
}
