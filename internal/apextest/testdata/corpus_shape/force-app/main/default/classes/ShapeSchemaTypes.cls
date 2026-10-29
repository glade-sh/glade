public class ShapeSchemaTypes extends ShapeSchemaBase {
    // Compile-only: Schema-qualified standard and setup objects as field,
    // parameter, local, constructor, return, generic-argument and
    // inherited-base types.
    public static Schema.Contact shared = new Schema.Contact();
    public Schema.Group queue;
    public List<Schema.Group> queues = new List<Schema.Group>();
    public Map<Id, Schema.Group> queueById = new Map<Id, Schema.Group>();

    public Boolean observe() {
        Schema.Contact local = new Schema.Contact();
        Schema.Account account = new Schema.Account(Name = 'Shape');
        Schema.Group grouping = new Schema.Group(Name = 'Shape', Type = 'Queue');
        Schema.User person = new Schema.User(LastName = 'Shape');
        List<Schema.Group> groups = new List<Schema.Group>{ grouping };
        Map<Id, Schema.User> users = new Map<Id, Schema.User>();
        parent = account;
        queue = grouping;
        return item.Account == null && local.Account == null && check(account) && make().Account == null &&
            pick(grouping).Name == 'Shape' && groups.size() == 1 && users.isEmpty() &&
            baseGroup.Name == null && baseUsers.isEmpty() && person.LastName == 'Shape';
    }

    public static Boolean check(Schema.Account value) {
        return value.Name == 'Shape';
    }

    public static Schema.Contact make() {
        return shared;
    }

    public static Schema.Group pick(Schema.Group value) {
        return value;
    }
}
