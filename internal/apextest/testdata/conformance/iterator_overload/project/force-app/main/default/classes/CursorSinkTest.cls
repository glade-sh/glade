@IsTest private class CursorSinkTest {
    @IsTest static void concreteCursor() {
        System.assert(false, 'OBSERVED|concreteCursor|' + JSON.serialize(new Map<String,Object>{'count'=>CursorSink.run()}));
    }
    @IsTest static void contactCursor() {
        Iterator<Contact> cursor = new List<Contact>{new Contact(LastName='Seed'),new Contact(LastName='Leaf')}.iterator();
        System.assert(false, 'OBSERVED|contactCursor|' + JSON.serialize(new Map<String,Object>{'count'=>CursorSink.count(cursor)}));
    }
    @IsTest static void instanceCursor() {
        Iterator<Account> cursor = new List<Account>{new Account(Name='Instance')}.iterator();
        System.assert(false, 'OBSERVED|instanceCursor|' + JSON.serialize(new Map<String,Object>{'count'=>new CursorSink().accept(cursor)}));
    }
    @IsTest static void exactCursor() {
        Iterator<Object> cursor = new List<Object>{7,8}.iterator();
        System.assert(false, 'OBSERVED|exactCursor|' + JSON.serialize(new Map<String,Object>{'count'=>CursorSink.count(cursor)}));
    }
}
