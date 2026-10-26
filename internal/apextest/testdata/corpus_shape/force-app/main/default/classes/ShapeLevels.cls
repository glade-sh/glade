public virtual class ShapeLevels {
    public enum Level { NONE, NOTICE, ALERT }
    public enum Status { OPEN, CLOSED }
    public enum Message { NOTE }
    // Compile-only: exercise every enum receiver form without inventing values.
    public static void compileRoutes() {
        String label = Level.NONE.name();
        Integer position = Level.NONE.ordinal();
        Level parsed = Level.valueOf('NONE');
        List<Level> allLevels = Level.values();
        Boolean same = parsed == Level.NONE;
        String qualified = ShapeLevels.Level.NONE.name();
        List<Status> statuses = Status.values();
        Status open = Status.valueOf('OPEN');
        Message note = Message.valueOf('NOTE');
        Integer messages = Message.values().size();
        ShapeLevelsChild.childRoutes();
        new ShapeInnerTargets.Leaf().matches();
        ShapeInnerTargets.castTarget(new ShapeInnerTargets.Leaf());
        new ShapeInnerPeer.Leaf().matches();
        // Native permission Id call: conformance/dispatch_followups/org.tsv:12-13.
        // Both spellings must reach the class constant's instance method.
        Id permissionSetId = null;
        try { AccessLevel bare = AccessLevel.USER_MODE.withPermissionSetId(permissionSetId); } catch (Exception e) {}
        try { System.AccessLevel qualifiedAccess = System.AccessLevel.USER_MODE.withPermissionSetId(permissionSetId); } catch (Exception e) {}
        Account.SObjectType.getDescribe();
        Account.Name.getDescribe();
    }
}
