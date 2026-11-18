@IsTest
private class PsrAsyncTest {
    public static Integer qRuns = 0;
    public static Boolean qPerm;
    public static String qFls;
    public static Integer fRuns = 0;
    public static Boolean fPerm;
    public static Integer bRuns = 0;
    public static Boolean bPerm;
    public static Id recId;

    public class Q implements Queueable {
        public void execute(QueueableContext ctx) {
            qRuns++;
            qPerm = FeatureManagement.checkPermission('PsrPerm');
            qFls = PsrUtil.flsRead(recId);
        }
    }

    @future
    public static void futureWork() {
        fRuns++;
        fPerm = FeatureManagement.checkPermission('PsrPerm');
    }

    @TestSetup
    static void setup() {
        insert as system new PsrObj__c(Name = 'as', PsrField__c = 'v');
    }

    @IsTest
    static void queueableJob() {
        Id psId = PsrUtil.permSetId();
        recId = [SELECT Id FROM PsrObj__c WHERE Name = 'as' LIMIT 1].Id;
        String ins = 'ok';
        try {
            insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId);
        } catch (Exception e) {
            ins = 'ex:' + e.getTypeName() + ':' + e.getMessage();
        }
        Test.startTest();
        System.enqueueJob(new Q());
        Test.stopTest();
        System.assert(false, 'job=queueable;insert=' + ins + ';runs=' + qRuns + ';perm=' + qPerm + ';fls=' + qFls);
    }

    @IsTest
    static void futureJob() {
        Id psId = PsrUtil.permSetId();
        String ins = 'ok';
        try {
            insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId);
        } catch (Exception e) {
            ins = 'ex:' + e.getTypeName() + ':' + e.getMessage();
        }
        Test.startTest();
        futureWork();
        Test.stopTest();
        System.assert(false, 'job=future;insert=' + ins + ';runs=' + fRuns + ';perm=' + fPerm);
    }

    @IsTest
    static void batchJob() {
        Id psId = PsrUtil.permSetId();
        String ins = 'ok';
        try {
            insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId);
        } catch (Exception e) {
            ins = 'ex:' + e.getTypeName() + ':' + e.getMessage();
        }
        Test.startTest();
        Database.executeBatch(new PsrBatchJob(), 1);
        Test.stopTest();
        System.assert(false, 'job=batch;insert=' + ins + ';runs=' + PsrBatchJob.runs + ';perm=' + PsrBatchJob.sawPerm);
    }
}
