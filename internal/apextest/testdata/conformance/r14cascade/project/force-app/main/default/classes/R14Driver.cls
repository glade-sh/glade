public class R14Driver {
    public static Integer defaultSeen = 0;
    public static Integer explicitSeen = 0;
    public static void run() {
        Boolean missing = null;
        Boolean rejected = false;
        try {
            Boolean ignored = !missing;
        } catch (NullPointerException expected) {
            rejected = true;
        }
        System.assertEquals(true, rejected);
        defaultSeen = 0;
        explicitSeen = 0;
        insert new List<R14Order__c>{new R14Order__c(Name='default'), new R14Order__c(Name='explicit', Flag__c=true)};
        System.assertEquals(1, defaultSeen);
        System.assertEquals(1, explicitSeen);
    }
}
