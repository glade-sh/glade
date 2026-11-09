public class AccessLevelRenderProbe {
    public static Map<String,Object> out = new Map<String,Object>();
    public class Holder implements Queueable {
        public AccessLevel sysLevel = AccessLevel.SYSTEM_MODE;
        public AccessLevel usrLevel = AccessLevel.USER_MODE;
        public void execute(QueueableContext context) {
            AccessLevelRenderProbe.out.put('carriedSystemText', String.valueOf(sysLevel));
            AccessLevelRenderProbe.out.put('carriedUserText', String.valueOf(usrLevel));
            AccessLevelRenderProbe.out.put('carriedSystemIdentity', sysLevel == AccessLevel.SYSTEM_MODE);
            AccessLevelRenderProbe.out.put('carriedUserIdentity', usrLevel == AccessLevel.USER_MODE);
            AccessLevelRenderProbe.out.put('carriedSystemEquals', sysLevel.equals(AccessLevel.SYSTEM_MODE));
            AccessLevelRenderProbe.out.put('carriedSystemHash', sysLevel.hashCode() == AccessLevel.SYSTEM_MODE.hashCode());
            AccessLevelRenderProbe.out.put('carriedUserNotSystem', usrLevel != AccessLevel.SYSTEM_MODE);
        }
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        result.put('systemText', String.valueOf(AccessLevel.SYSTEM_MODE));
        result.put('userText', String.valueOf(AccessLevel.USER_MODE));
        result.put('systemIdentity', AccessLevel.SYSTEM_MODE == AccessLevel.SYSTEM_MODE);
        result.put('systemUserDiffer', AccessLevel.SYSTEM_MODE == AccessLevel.USER_MODE);
        result.put('systemEquals', AccessLevel.SYSTEM_MODE.equals(AccessLevel.SYSTEM_MODE));
        AccessLevel copy = AccessLevel.SYSTEM_MODE;
        result.put('copyIdentity', copy == AccessLevel.SYSTEM_MODE);
        List<AccessLevel> listed = new List<AccessLevel>{ AccessLevel.USER_MODE };
        result.put('listedIdentity', listed[0] == AccessLevel.USER_MODE);
        if (Test.isRunningTest()) {
            Test.startTest();
            System.enqueueJob(new Holder());
            Test.stopTest();
            result.putAll(AccessLevelRenderProbe.out);
        }
        return result;
    }
}
