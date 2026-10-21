public class EntryHooks {
    public interface VoidHook { void apply(List<SObject> rows); }
    public interface CountHook { Integer count(List<SObject> rows); }
}
