public class StubService {
    public StubDomain domain = new StubDomain();
    public void run(List<Thing__c> things) { domain.upsertThings(things); }
}
