public class ShadowExtra {
    public static Map<String,String> observe() {
        Map<String,String> out = new Map<String,String>();
        out.put('class', QueryException.class.getName());
        out.put('case', queryexception.class.getName());
        try { throw new System.QueryException('native'); }
        catch (queryexception e) { out.put('system', 'user'); }
        catch (Exception e) { out.put('system', 'exception'); }
        try { throw new queryexception('local'); }
        catch (QUERYEXCEPTION e) { out.put('local', 'user'); }
        catch (Exception e) { out.put('local', 'exception'); }
        return out;
    }
}
