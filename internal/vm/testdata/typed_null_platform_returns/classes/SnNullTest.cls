@isTest
private class SnNullTest {
    @isTest
    static void site() {
        SnNull.runSite();
        System.assertEquals('ROWS', String.join(SnNull.rows, ' | '));
    }
    @isTest
    static void session() {
        SnNull.runSession();
        System.assertEquals('ROWS', String.join(SnNull.rows, ' | '));
    }
    @isTest
    static void sessionNull() {
        SnNull.sessionNull();
        System.assertEquals('ROWS', String.join(SnNull.rows, ' | '));
    }
    @isTest
    static void sessionProfile() {
        SnNull.sessionProfile();
        System.assertEquals('ROWS', String.join(SnNull.rows, ' | '));
    }
    @isTest
    static void asyncOutside() {
        SnNull.runAsyncOutside();
        System.assertEquals('ROWS', String.join(SnNull.rows, ' | '));
    }
    @isTest
    static void asyncNoDelay() {
        Test.startTest();
        System.enqueueJob(new SnQueue('nodelay'));
        Test.stopTest();
        System.debug('SNROW async.nodelay.rows ' + SnQueue.rows);
        System.assertEquals('ROWS', String.join(SnQueue.rows, ' | '));
    }
    @isTest
    static void asyncDelay() {
        Test.startTest();
        System.enqueueJob(new SnQueue('delay1'), 1);
        Test.stopTest();
        System.debug('SNROW async.delay1.rows ' + SnQueue.rows);
        System.assertEquals('ROWS', String.join(SnQueue.rows, ' | '));
    }
    @isTest
    static void asyncOptions() {
        AsyncOptions o = new AsyncOptions();
        o.MinimumQueueableDelayInMinutes = 2;
        Test.startTest();
        System.enqueueJob(new SnQueue('options2'), o);
        Test.stopTest();
        System.debug('SNROW async.options2.rows ' + SnQueue.rows);
        System.assertEquals('ROWS', String.join(SnQueue.rows, ' | '));
    }
}
