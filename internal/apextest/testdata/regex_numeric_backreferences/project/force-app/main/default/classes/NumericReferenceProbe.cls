public class NumericReferenceProbe {
    public static Map<String,Object> observe(String separator, String input) {
        Map<String,Object> rows = new Map<String,Object>();
        String text = 'L'+input+'R'+input+'T';
        try { rows.put('String.split/default', text.split(separator)); }
        catch(Exception error) { rows.put('String.split/default',error.getTypeName()); }
        try { rows.put('String.split/negative', text.split(separator,-1)); }
        catch(Exception error) { rows.put('String.split/negative',error.getTypeName()); }
        try { rows.put('String.split/positive', text.split(separator,2)); }
        catch(Exception error) { rows.put('String.split/positive',error.getTypeName()); }
        try { rows.put('Pattern.split/default',Pattern.compile(separator).split(text)); }
        catch(Exception error) { rows.put('Pattern.split/default',error.getTypeName()); }
        try { rows.put('Pattern.split/negative',Pattern.compile(separator).split(text,-1)); }
        catch(Exception error) { rows.put('Pattern.split/negative',error.getTypeName()); }
        try { rows.put('Pattern.split/positive',Pattern.compile(separator).split(text,2)); }
        catch(Exception error) { rows.put('Pattern.split/positive',error.getTypeName()); }
        try { rows.put('String.replaceAll', text.replaceAll(separator,'Q')); }
        catch(Exception error) { rows.put('String.replaceAll',error.getTypeName()); }
        try { rows.put('Matcher.replaceAll',Pattern.compile(separator).matcher(text).replaceAll('Q')); }
        catch(Exception error) { rows.put('Matcher.replaceAll',error.getTypeName()); }
        try { rows.put('Matcher.matches',Pattern.compile(separator).matcher(input).matches()); }
        catch(Exception error) { rows.put('Matcher.matches',error.getTypeName()); }
        try {
            Matcher search=Pattern.compile(separator).matcher(text);
            List<String> found=new List<String>();
            while(search.find()) { found.add(search.group()); }
            rows.put('Matcher.find',found);
            rows.put('Matcher.groupCount',search.groupCount());
        } catch(Exception error) { rows.put('Matcher.find',error.getTypeName()); }
        return rows;
    }
}
