public class FernThrow {
    public class BoomException extends Exception {}
    @InvocableMethod
    public static void run(List<Fern_Thr__c> rows) { throw new BoomException('boom action'); }
}
