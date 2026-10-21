public class LiteralShapes {
    public class Part {}
    public enum Color { RED }
    public static String TOKEN = 'x';
    public static String setting { get; set; }
    public String member;
    public void action() {}
    public static Map<String, Object> observe() {
        Type outerMissing = LiteralShapes.Unknown.class;
        Type nestedMissing = LiteralShapes.Part.Unknown.class;
        Type deepMissing = LiteralShapes.Unknown.Deeper.class;
        Type instanceMember = LiteralShapes.member.class;
        Type instanceMethod = LiteralShapes.action.class;
        Type enumMissing = LiteralShapes.Color.Unknown.class;
        Type standardMissing = Account.Unknown.class;
        Type mixedCase = literalshapes.UNKNOWN.class;
        Type existing = LiteralShapes.Part.class;
        return new Map<String, Object>{'outerMissing'=>outerMissing.getName(), 'nestedMissing'=>nestedMissing.getName(),
            'deepMissing'=>deepMissing.getName(), 'instanceMember'=>instanceMember.getName(), 'instanceMethod'=>instanceMethod.getName(),
            'enumMissing'=>enumMissing.getName(), 'standardMissing'=>standardMissing.getName(),
            'mixedCase'=>mixedCase.getName(), 'existing'=>existing.getName()};
    }
}
