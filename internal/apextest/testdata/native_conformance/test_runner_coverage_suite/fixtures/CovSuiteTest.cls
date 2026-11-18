@IsTest
private class CovSuiteTest {
    @IsTest
    static void multiLine() {
        System.assertEquals(6, CovMultiLine.total(1, 2, 3));
        System.assertEquals('a-b-c', CovMultiLine.join('a', 'b', 'c'));
    }

    @IsTest
    static void ternaryBothBranches() {
        System.assertEquals('big', CovTernary.pick(20));
        System.assertEquals('small', CovTernary.pick(1));
        System.assertEquals(0, CovTernary.clamp(-5));
        System.assertEquals(5, CovTernary.clamp(5));
    }

    @IsTest
    static void ternaryMissOnly() {
        System.assertEquals('miss', CovTernary.never(1));
    }

    @IsTest
    static void switchEnumAll() {
        System.assertEquals('open', CovSwitchEnum.describe(CovSwitchEnum.Phase.OPEN));
        System.assertEquals('hold', CovSwitchEnum.describe(CovSwitchEnum.Phase.HOLD));
        System.assertEquals('closed', CovSwitchEnum.describe(CovSwitchEnum.Phase.CLOSED));
    }

    @IsTest
    static void switchSObjectAll() {
        System.assertEquals('account', CovSwitchSObject.kind(new Account(Name = 'Cov Kind')));
        System.assertEquals('contact', CovSwitchSObject.kind(new Contact(LastName = 'Cov Kind')));
        System.assertEquals('other', CovSwitchSObject.kind(new Lead(Company = 'Cov Kind', LastName = 'Cov Kind')));
    }

    @IsTest
    static void tryCatchNoCatch() {
        System.assertEquals(5, CovTryCatch.safeDivide(10, 2));
    }

    @IsTest
    static void innerClasses() {
        System.assertEquals(8, CovInner.useNested(4));
        System.assertEquals('COV', new CovInner().useInstance('cov'));
    }

    @IsTest
    static void staticInit() {
        System.assertEquals(14, CovStaticInit.value());
        System.assertEquals(7, CovStaticInit.seed);
    }

    @IsTest
    static void properties() {
        CovProps p = CovProps.build('cov');
        System.assertEquals(3, p.Count);
        System.assertEquals('COV', p.Upper);
        System.assertEquals('fixed', p.NeverSet);
    }

    @IsTest
    static void shapes() {
        System.assertEquals(9, CovShapes.run(3));
    }

    @IsTest
    static void triggerHandler() {
        insert new Account(Name = 'Cov Trigger One');
        insert new Account(Name = 'Cov Trigger Two');
        System.assertEquals(2, CovTriggerHandler.seen);
    }

    @IsTest
    static void testVisibleMembers() {
        System.assertEquals(2, CovTestVisible.bump(1));
        System.assertEquals('cov', new CovTestVisible().echo('cov'));
        System.assertEquals(6, CovTestVisible.exposed(3));
    }

    @IsTest
    static void failingPath() {
        System.assertEquals(2, CovFailingPath.touch(1));
        System.assert(false, 'intentional failure so this path is only reached by a failing test');
    }

    @IsTest
    static void earlyReturnOnly() {
        System.assertEquals('early', CovEarlyReturn.route(-1));
    }
}
