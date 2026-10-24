@IsTest private class ShapeProjectTest {
    // These methods exercise executable project routes without asserting new output values.
    @IsTest static void enumReceivers() { ShapeLevels.compileRoutes(); }
    // Native: inner_type_shapes/org.tsv:72-73 (@IsTest inner subclass).
    private class Fake extends ShapeInnerTargets {}
    @IsTest static void lexicalSystemNames() {
        ShapeShadow.compileRoutes();
        ShapeInnerTargets value = new Fake();
        System.assert(value instanceof Fake);
    }
    @IsTest static void staticInstanceReceivers() { ShapeOuter.compileRoutes(); }
    // Native: conformance/change_event_describe/native/api62/anonymous/org.tsv:2
    // and native/api67/anonymous/org.tsv:2; their ORG-RUN.txt capture notes.
    // Those describe captures used an API 68 class at endpoints 62 and 67.
    @IsTest static void changeEventIndustrySurface() {
        System.assert(AccountChangeEvent.SObjectType.getDescribe().fields.getMap().containsKey('industry'));
    }
    @IsTest static void selectorDomainWorkDispatcher() {
        ShapeWork work = new ShapeWork();
        work.registerNew(new Account(Name = 'shape-work'));
        work.commitWork();
        ShapeSelector selector = new ShapeSelector();
        Schema.SObjectType objectType = selector.objectType();
        List<Schema.SObjectField> fields = selector.fields();
        List<Account> found = selector.selectByIds(new Map<Id, Account>([SELECT Id FROM Account]).keySet());
    }
    // Native: conformance/scheduled_nesting_stop/org.tsv:8,27; ORG-RUN.txt.
    @IsTest static void topLevelSchedulableAtStopTest() {
        Test.startTest();
        System.schedule('shape top', '0 0 0 1 1 ? 2099', new ShapeSchedule());
        Test.stopTest();
        System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'shape-top']);
    }
    // Native: conformance/scheduled_nesting_stop/org.tsv:19,38; ORG-RUN.txt.
    @IsTest static void innerSchedulableAtStopTest() {
        Test.startTest();
        System.schedule('shape inner', '0 0 0 1 1 ? 2099', new ShapePlanner.Tick());
        Test.stopTest();
        System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'shape-inner']);
    }
    // Native: conformance/scheduled_nesting_stop/org.tsv:4,23; ORG-RUN.txt.
    // Only the first batch side effect is pinned; chained queue delivery is compile-only.
    @IsTest static void batchToQueueable() {
        insert new Account(Name = 'shape-seed');
        Test.startTest();
        Database.executeBatch(new ShapeBatch(), 1);
        Test.stopTest();
        System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'shape-batch']);
    }
    // Native: conformance/scheduled_nesting_stop/org.tsv:10,29; ORG-RUN.txt.
    // Only the submitted queueable is pinned; its child enqueue is a project shape check.
    @IsTest static void queueableToQueueable() {
        Test.startTest();
        System.enqueueJob(new ShapeQueue(false));
        Test.stopTest();
        System.assert([SELECT COUNT() FROM Account WHERE Name = 'shape-queue'] > 0);
    }
    // Native: dated_dispatch_cache/org.tsv:2-5 (inner instanceof/cast),
    // and inner_type_shapes/org.tsv:34-41 (simple-name targets return true).
    @IsTest static void innerSubclassTargets() {
        System.assert(new ShapeInnerTargets.Leaf().matches());
        System.assert(ShapeInnerTargets.castTarget(new ShapeInnerTargets.Leaf()));
    }
    // Native: inner_type_scope/org.tsv:2-3 (org_project), and
    // inner_type_shapes/org.tsv:50-53 (empty publish result has size zero).
    @IsTest static void unrelatedEventBus() {
        System.assertEquals(0, ShapePublisher.publishEmpty());
    }
    // Native: inner_type_shapes/org.tsv:64-67 (both lexical Leaf targets match).
    @IsTest static void duplicateInnerNames() {
        System.assert(new ShapeInnerTargets.Leaf().matches());
        System.assert(new ShapeInnerPeer.Leaf().matches());
    }
    // Native: inner_type_shapes/org.tsv:18-21 (Type.B.name() yields B).
    @IsTest static void systemNamedEnumConstant() {
        System.assertEquals('B', ShapeShadow.typeName());
    }
    // Native: inner_type_shapes/org.tsv:72-73 (private @IsTest inner subclass).
    @IsTest static void testInnerSubclass() {
        ShapeInnerTargets value = new Fake();
        System.assert(value instanceof Fake);
    }
}
