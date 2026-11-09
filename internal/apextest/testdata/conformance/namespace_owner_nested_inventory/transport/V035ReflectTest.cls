@IsTest private class V035ReflectTest {
    @IsTest static void observe() {
        String r = 'childDotted=' + {{NAMESPACE}}.V035Owner.attempt(Type.forName('{{NAMESPACE}}.V035Owner.Child'))
            + ';childTwoArg=' + {{NAMESPACE}}.V035Owner.attempt(Type.forName('{{NAMESPACE}}', 'V035Owner.Child'))
            + ';childUnqualified=' + {{NAMESPACE}}.V035Owner.attempt(Type.forName('V035Owner.Child'))
            + ';pubDotted=' + {{NAMESPACE}}.V035Owner.attempt(Type.forName('{{NAMESPACE}}.V035Owner.Pub'))
            + ';quietDotted=' + {{NAMESPACE}}.V035Owner.attempt(Type.forName('{{NAMESPACE}}.V035Owner.Quiet'))
            + ';pubLiteral=' + {{NAMESPACE}}.V035Owner.attempt({{NAMESPACE}}.V035Owner.Pub.class)
            + ';missingDotted=' + {{NAMESPACE}}.V035Owner.attempt(Type.forName('{{NAMESPACE}}.V035Missing'))
            + ';qualifiedLeaf=' + new {{NAMESPACE}}.V035Outer.Leaf().y()
            + ';lowercaseLeaf=' + new {{namespace}}.v035outer.leaf().y();
        System.assert(false, 'OBSERVED|reflect|' + r);
    }
}
