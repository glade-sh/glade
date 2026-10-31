public class DependencyAction {
    public static Integer calls = 0;
    @InvocableMethod(label='Dependency action') public static void apply() { calls++; }
}
