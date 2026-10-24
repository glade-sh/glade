public class NumericRouteObservation {
    public static Map<String,Object> observe() {
        Map<String,Object> rows = new Map<String,Object>();
        for (String text : new List<String>{'0.0.0.0','1.2.3.4','127.0.0.1','192.168.1.254','255.255.255.255','256.0.0.0','-1.0.0.0','0.0.0','x.0.0.0','2147483648.0.0.0'}) {
            rows.put('Integer.split/'+text, IntegerSplitAddressProbe.valid(text));
            rows.put('Matcher.groups/'+text, GroupedAddressProbe.valid(text));
        }
        return rows;
    }
    public static Map<String,Object> grapheme() {
        Map<String,Object> rows = new Map<String,Object>();
        String mark=String.fromCharArray(new List<Integer>{769});
        String text='e'+mark;
        String separator='e'+String.fromCharArray(new List<Integer>{92})+'b{g}';
        try { rows.put('String.split',text.split(separator,-1)); }
        catch(Exception error) { rows.put('String.error',error.getTypeName()); }
        try { rows.put('Pattern.split',Pattern.compile(separator).split(text,-1)); }
        catch(Exception error) { rows.put('Pattern.error',error.getTypeName()); }
        return rows;
    }
}
