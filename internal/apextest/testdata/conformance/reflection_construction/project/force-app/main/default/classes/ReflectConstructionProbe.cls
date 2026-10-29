public class ReflectConstructionProbe {
    public class Plain { public Integer value = 7; }
    public class PrivateConstructor { private PrivateConstructor() {} }
    public virtual class ProtectedConstructor { protected ProtectedConstructor() {} }
    public class ArgumentConstructor { public ArgumentConstructor(Integer value) {} }
    public abstract class AbstractValue {}
    public interface Contract {}
    public class ConstructionException extends Exception {}
    public class ThrowingConstructor {
        public ThrowingConstructor() { throw new ConstructionException('construction interrupted'); }
    }
    public static String capture(Type target) {
        Map<String, Object> out = new Map<String, Object>{'missing' => target == null};
        try {
            if (target != null) out.put('type', target.getName());
            Object value = target.newInstance();
            out.put('created', value != null);
            if (value instanceof Plain) out.put('value', ((Plain)value).value);
        } catch (Exception error) {
            out.put('error', error.getTypeName());
            out.put('message', error.getMessage());
        }
        return JSON.serialize(out);
    }
}
