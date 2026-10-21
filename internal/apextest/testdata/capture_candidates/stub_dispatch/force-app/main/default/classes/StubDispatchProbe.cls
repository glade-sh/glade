public class StubDispatchProbe {
    public static Integer bodyCalls = 0;
    public static Integer constructorCalls = 0;
    public static Integer providerCalls = 0;
    public static Integer httpCalls = 0;
    public static String lastMethod = '';
    public static Integer lastArity = -1;
    public static Boolean failConstruction = false;
    public static StubDispatchTarget held;
    public static StubDispatchProvider heldProvider;
    public class ProbeFailureException extends Exception {}

    private class HttpReply implements HttpCalloutMock {
        public HttpResponse respond(HttpRequest request) {
            httpCalls++;
            HttpResponse response = new HttpResponse();
            response.setStatusCode(200);
            response.setBody('http marker');
            return response;
        }
    }
    private static void reset() {
        bodyCalls = 0;
        constructorCalls = 0;
        providerCalls = 0;
        httpCalls = 0;
        lastMethod = '';
        lastArity = -1;
        failConstruction = false;
        held = null;
        heldProvider = null;
    }
    private static String snapshot() {
        return '|body=' + bodyCalls + '|ctor=' + constructorCalls +
            '|provider=' + providerCalls + '|http=' + httpCalls +
            '|method=' + lastMethod + '|arity=' + lastArity;
    }
    private static StubDispatchTarget stub(StubDispatchProvider provider) {
        return (StubDispatchTarget)Test.createStub(StubDispatchTarget.class, provider);
    }
    private static StubDispatchTarget fromStatic() {
        return held;
    }
    private static Integer storedRows() {
        List<SObject> values = Database.query(
            'SELECT Id FROM Account WHERE Name = \'capture-row\'');
        return values.size();
    }
    private static String flags(Schema.SObjectField token) {
        Schema.DescribeFieldResult value = token.getDescribe();
        return 'idLookup=' + value.isIdLookup() + '|externalId=' +
            value.isExternalId() + '|unique=' + value.isUnique();
    }
    public static String run(String id) {
        reset();
        try {
            return execute(id) + snapshot();
        } catch (Exception error) {
            return 'EXC|' + error.getTypeName() + '|' + error.getMessage() + snapshot();
        }
    }
    private static String execute(String id) {
        if (id == 'A01') return flags(Opportunity.Id);
        if (id == 'A02') return flags(Opportunity.StageName);
        if (id == 'A03') return flags(User.Username);
        if (id == 'A04') return flags(Opportunity.AccountId);

        StubDispatchProvider provider = new StubDispatchProvider();
        if (id == 'H01') {
            provider.answer = 'stub marker';
            return String.valueOf(stub(provider).fetch());
        }
        if (id == 'H02') return String.valueOf(stub(provider).fetch());
        if (id == 'H03') {
            // An anonymous control must never attempt a real network request.
            if (!Test.isRunningTest()) return 'BOUNDARY|test context required';
            return new StubDispatchTarget().fetch();
        }
        if (id == 'H04') {
            Test.setMock(HttpCalloutMock.class, new HttpReply());
            return new StubDispatchTarget().fetch();
        }
        if (id == 'H05') {
            provider.answer = 'static marker';
            heldProvider = provider;
            held = stub(provider);
            Map<String, List<StubDispatchTarget>> aliases =
                new Map<String, List<StubDispatchTarget>>{
                    'service' => new List<StubDispatchTarget>{fromStatic()}};
            return aliases.get('service')[0].fetch();
        }
        if (id == 'H06') {
            provider.answer = 'initial marker';
            StubDispatchTarget target = stub(provider);
            StubDispatchProvider other = new StubDispatchProvider();
            other.Verifying = true;
            other.answer = 'other marker';
            provider.answer = 'updated marker';
            return target.fetch();
        }
        if (id == 'H07') {
            provider.keyed = true;
            provider.answers.put(new StubDispatchCallKey('fetch', 0), 'key marker');
            return stub(provider).fetch();
        }
        if (id == 'H08') {
            Test.setMock(HttpCalloutMock.class, new HttpReply());
            Test.startTest();
            String value = new StubDispatchTarget().fetch();
            Test.stopTest();
            return value;
        }

        if (id == 'D01') return String.valueOf(stub(provider).objectType() == null);
        if (id == 'D02') return stub(provider).objectType().getDescribe().getName();
        if (id == 'D03') {
            provider.answer = Account.SObjectType;
            return stub(provider).objectType().getDescribe().fields.getMap()
                .get('Name').getDescribe().getName();
        }
        if (id == 'D04') return String.valueOf(stub(provider).fieldList() == null);
        if (id == 'D05') return String.valueOf(stub(provider).fieldList().size());
        if (id == 'D06') {
            provider.answer = new List<Schema.SObjectField>{Account.Id, Account.Name};
            List<Schema.SObjectField> fields = stub(provider).fieldList();
            return fields.size() + '|' + fields[0].getDescribe().getName();
        }
        if (id == 'D07') return String.valueOf(stub(provider).getDecimalPlaces());
        if (id == 'D08') {
            StubDispatchTarget target = new StubDispatchTarget();
            return target.objectType().getDescribe().getName() + '|' +
                target.fieldList().size();
        }
        if (id == 'D09') {
            StubDispatchTarget target = stub(provider);
            return String.valueOf(target.fluent() == null);
        }

        if (id == 'L01') {
            provider.answer = new List<Account>();
            List<Contact> values = (List<Contact>)stub(provider).rows();
            return String.valueOf(values.size());
        }
        if (id == 'L02') {
            provider.answer = new List<Account>{new Account(Name = 'capture-row')};
            List<Contact> values = (List<Contact>)stub(provider).rows();
            return String.valueOf(values.size());
        }
        if (id == 'L03') {
            provider.answer = new List<SObject>();
            List<Contact> values = (List<Contact>)stub(provider).rows();
            return String.valueOf(values.size());
        }
        if (id == 'L04') {
            provider.answer = Database.query(
                'SELECT Id FROM Account WHERE Name = \'capture-empty\'');
            List<Contact> values = (List<Contact>)stub(provider).rows();
            return String.valueOf(values.size());
        }
        if (id == 'L05') {
            insert new Account(Name = 'capture-row');
            provider.answer = Database.query(
                'SELECT Id FROM Account WHERE Name = \'capture-row\'');
            List<Contact> values = (List<Contact>)stub(provider).rows();
            return String.valueOf(values.size());
        }
        if (id == 'L06') {
            insert new Account(Name = 'capture-row');
            provider.answer = Database.query(
                'SELECT Id, Name FROM Account WHERE Name = \'capture-row\'');
            List<Account> values = (List<Account>)stub(provider).rows();
            return values.size() + '|' + values[0].Name;
        }

        if (id == 'E01') {
            stub(provider).write(new List<SObject>{new Account(Name = 'capture-row')});
            return String.valueOf(storedRows());
        }
        if (id == 'E02') {
            new StubDispatchTarget().write(
                new List<SObject>{new Account(Name = 'capture-row')});
            return String.valueOf(storedRows());
        }
        if (id == 'E03' || id == 'E04') {
            heldProvider = provider;
            if (id == 'E03') held = stub(provider);
            try {
                throw new ProbeFailureException('callback marker');
            } catch (ProbeFailureException original) {
                fromStatic().write(
                    new List<SObject>{new Account(Name = 'capture-row')});
            }
            return String.valueOf(storedRows());
        }
        if (id == 'E05') {
            provider.answer = 'cycle marker';
            heldProvider = provider;
            held = stub(provider);
            provider.proxies.add(held);
            return fromStatic().fetch() + '|' + provider.proxies[0].fetch();
        }
        if (id == 'E06') {
            provider.answer = 'first marker';
            held = stub(provider);
            provider.Verifying = true;
            provider.answer = 'second marker';
            return fromStatic().fetch();
        }
        if (id == 'E07') {
            StubDispatchTarget ordinary = new StubDispatchTarget();
            return String.valueOf(ordinary.records.size());
        }
        if (id == 'E08') {
            failConstruction = true;
            return String.valueOf(new StubDispatchTarget().records.size());
        }
        if (id == 'E09') {
            StubDispatchTarget target = stub(provider);
            return target == null ? 'null target' : 'stub created';
        }
        throw new ProbeFailureException('unknown row');
    }
}
