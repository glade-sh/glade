@JsonAccess(serializable='never' deserializable='never')
public class RuntimeJSONSelfBlocked {
    public String value = 'self';
    public static String observeSelf() {
        String encoded;
        try { encoded = JSON.serialize(new RuntimeJSONSelfBlocked()); }
        catch (Exception e) { encoded = e.getTypeName() + '|' + e.getMessage(); }
        String decoded;
        try { decoded = ((RuntimeJSONSelfBlocked) JSON.deserialize('{"value":"decoded"}', RuntimeJSONSelfBlocked.class)).value; }
        catch (Exception e) { decoded = e.getTypeName() + '|' + e.getMessage(); }
        return encoded + ';' + decoded;
    }
    public String observeInstance() {
        String encoded;
        try { encoded = JSON.serialize(this); }
        catch (Exception e) { encoded = e.getTypeName() + '|' + e.getMessage(); }
        String decoded;
        try { decoded = ((RuntimeJSONSelfBlocked) JSON.deserialize('{"value":"decoded"}', RuntimeJSONSelfBlocked.class)).value; }
        catch (Exception e) { decoded = e.getTypeName() + '|' + e.getMessage(); }
        return encoded + ';' + decoded;
    }
    public String observeOtherInstance() {
        String encoded;
        try { encoded = JSON.serialize(new RuntimeJSONSelfBlocked()); }
        catch (Exception e) { encoded = e.getTypeName() + '|' + e.getMessage(); }
        String decoded;
        try { decoded = ((RuntimeJSONSelfBlocked) JSON.deserialize('{"value":"decoded"}', RuntimeJSONSelfBlocked.class)).value; }
        catch (Exception e) { decoded = e.getTypeName() + '|' + e.getMessage(); }
        return encoded + ';' + decoded;
    }
}
