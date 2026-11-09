public virtual class OwnedBroad {
    public Schema.Account accountField;
    public Schema.Contact contactField;
    public Schema.User userField;
    public Schema.Group groupField;
    public enum Phase { FIRST, SECOND }
    public class Request { public String value = 'owned'; }
    public class Response { public String value; }
    public virtual class Message { public String value; }
    public class Item { public Integer count; }
    public interface Type { String label(); }
    public class Impl implements Type { public String label() { return 'owned'; } }
    public class OwnedException extends Exception {}
    public static Request request = new Request();
    public static String readStatic() { return OwnedBroad.request.value; }
    public Schema.Group consume(Schema.Account a, Schema.Contact c, Schema.User u, Schema.Group g) {
        List<Schema.Account> accounts = new List<Schema.Account>{a};
        Map<Id,SObject> records = new Map<Id,SObject>();
        Schema.Account localA = new Schema.Account();
        Schema.Contact localC = new Schema.Contact();
        Schema.User localU = new Schema.User();
        Schema.Group localG = new Schema.Group();
        Phase p = Phase.valueOf('FIRST');
        List<Phase> values = Phase.values();
        String n = p.name(); Integer ord = p.ordinal();
        switch on p { when FIRST { n = 'first'; } when SECOND { n = 'second'; } }
        Object candidate = new Item();
        if (candidate instanceof Item) { Item castItem = (Item) candidate; }
        try { throw new OwnedException(); } catch (OwnedException ex) { n = ex.getMessage(); }
        return g;
    }
    public class ChildMessage extends Message {}
    public class Sibling { public Request request; public Response response; public Type contract; public ChildMessage message; }
}
