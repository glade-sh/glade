public class ReflectionEdgeHelper {
    public static String describeType(Type token) {
        String displayed = String.valueOf(token);
        String element = displayed.substringAfter('<').substringBefore('>');
        return displayed + '|' + token.getName() + '|' + (Type.forName(element) != null);
    }
}
