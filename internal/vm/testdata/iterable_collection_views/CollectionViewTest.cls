@IsTest private class CollectionViewTest {
    private static Object pass(Iterable<Object> values) { return values; }
    @IsTest static void preservesSetType() {
        Object result = pass(new Set<String>{'Alpha'});
        System.assert(result instanceof Set<String>);
        System.assert(!(result instanceof Set<Integer>));
    }
    @IsTest static void preservesListType() {
        Object result = pass(new List<String>{'Alpha'});
        System.assert(result instanceof List<String>);
        System.assert(!(result instanceof List<Integer>));
    }
    @IsTest static void preservesEmptyTypes() {
        System.assert(pass(new Set<String>()) instanceof Set<String>);
        System.assert(pass(new List<String>()) instanceof List<String>);
    }
    @IsTest static void rejectsIntegerSetDowncast() {
        Object result = pass(new Set<Integer>{1});
        try { Set<String> narrowed = (Set<String>)result; System.assert(false); }
        catch (TypeException expected) { System.assert(expected.getMessage().contains('Invalid conversion')); }
    }
    @IsTest static void keepsObjectListType() {
        Object result = pass(new List<Object>{'Alpha'});
        System.assert(!(result instanceof List<String>));
    }
}
