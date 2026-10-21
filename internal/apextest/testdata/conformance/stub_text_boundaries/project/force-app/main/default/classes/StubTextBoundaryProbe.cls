public class StubTextBoundaryProbe {
    public class Holder {
        public Object value;
        public Holder(Object input) { value = input; }
    }
    public static Object pass(Object input) { return input; }
    public static String runtimeDescription(Object input) {
        try {
            Datetime impossible = (Datetime)input;
        } catch (Exception problem) {
            return problem.getMessage();
        }
        return 'cast-succeeded';
    }
    private static Map<String,Object> failure(Exception problem) {
        return new Map<String,Object>{'type'=>problem.getTypeName(), 'message'=>problem.getMessage()};
    }
    public static Map<String,Object> observe() {
        Map<String,Object> rows = new Map<String,Object>();
        JSONParser parser = JSON.createParser('{"mark":true}');
        try { rows.put('parserJson', JSON.serialize(parser)); }
        catch (Exception problem) { rows.put('parserJson', failure(problem)); }
        try { rows.put('parserPretty', JSON.serializePretty(parser)); }
        catch (Exception problem) { rows.put('parserPretty', failure(problem)); }
        try { rows.put('parserSuppressed', JSON.serialize(parser, true)); }
        catch (Exception problem) { rows.put('parserSuppressed', failure(problem)); }

        List<Contact> contacts = new List<Contact>{new Contact(LastName='Meadow')};
        rows.put('contactList', runtimeDescription(contacts));
        rows.put('contactListPassed', runtimeDescription(pass(contacts)));
        Holder holder = new Holder(contacts);
        rows.put('contactListField', runtimeDescription(holder.value));
        List<Object> erased = (List<Object>)contacts;
        rows.put('contactListView', runtimeDescription(erased));
        rows.put('objectListControl', runtimeDescription(new List<Object>{'leaf'}));
        rows.put('contactControl', runtimeDescription(new Contact(LastName='Control')));

        MeadowTextProvider provider = new MeadowTextProvider();
        try {
            MeadowTextSource proxy = (MeadowTextSource)Test.createStub(MeadowTextSource.class, provider);
            rows.put('interfaceDirect', proxy.toString());
            rows.put('interfaceDirectCalls', provider.calls);
            rows.put('interfaceValueOf', String.valueOf(proxy));
            rows.put('interfaceValueOfCalls', provider.calls);
            rows.put('interfaceConcat', '' + proxy);
            rows.put('interfaceConcatCalls', provider.calls);
            rows.put('interfaceFormat', String.format('{0}', new List<Object>{proxy}));
            rows.put('interfaceFormatCalls', provider.calls);
        } catch (Exception problem) { rows.put('interfaceStub', failure(problem)); }
        provider = new MeadowTextProvider();
        try {
            MeadowTextClass proxy = (MeadowTextClass)Test.createStub(MeadowTextClass.class, provider);
            rows.put('classDirect', proxy.toString());
            rows.put('classDirectCalls', provider.calls);
            rows.put('classValueOf', String.valueOf(proxy));
            rows.put('classValueOfCalls', provider.calls);
            rows.put('classConcat', '' + proxy);
            rows.put('classConcatCalls', provider.calls);
            rows.put('classFormat', String.format('{0}', new List<Object>{proxy}));
            rows.put('classFormatCalls', provider.calls);
        } catch (Exception problem) { rows.put('classStub', failure(problem)); }
        try {
            Object proxy = Test.createStub(HttpRequest.class, new MeadowTextProvider());
            rows.put('platformStub', 'created');
        } catch (Exception problem) { rows.put('platformStub', failure(problem)); }
        try {
            Object proxy = Test.createStub(MeadowLockedSource.class, new MeadowTextProvider());
            rows.put('lockedStub', 'created');
        } catch (Exception problem) { rows.put('lockedStub', failure(problem)); }
        return rows;
    }
}
