public class NameResolutionPinProbe {
    public class TypeNamePinToken {
        public String identify() { return 'inner'; }
    }

    public static String innerBeforeTopLevel() {
        return new TypeNamePinToken().identify();
    }

    public static String projectClassBeforePlatform() {
        return new URL().identify();
    }
}
