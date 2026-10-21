public virtual class StubDispatchTarget {
    public List<SObject> records;
    public StubDispatchTarget() {
        StubDispatchProbe.constructorCalls++;
        records = new List<SObject>();
        if (StubDispatchProbe.failConstruction) {
            throw new StubDispatchProbe.ProbeFailureException('construction marker');
        }
    }
    public virtual String fetch() {
        StubDispatchProbe.bodyCalls++;
        if (!Test.isRunningTest()) {
            throw new StubDispatchProbe.ProbeFailureException('transport requires test context');
        }
        HttpRequest request = new HttpRequest();
        request.setEndpoint('https://example.invalid/capture-only');
        request.setMethod('GET');
        return new Http().send(request).getBody();
    }
    public virtual Schema.SObjectType objectType() {
        StubDispatchProbe.bodyCalls++;
        return Account.SObjectType;
    }
    public virtual List<Schema.SObjectField> fieldList() {
        StubDispatchProbe.bodyCalls++;
        return new List<Schema.SObjectField>{Account.Id, Account.Name};
    }
    public virtual List<SObject> rows() {
        StubDispatchProbe.bodyCalls++;
        return Database.query('SELECT Id, Name FROM Account WHERE Name = \'capture-row\'');
    }
    public virtual Integer getDecimalPlaces() {
        StubDispatchProbe.bodyCalls++;
        return 7;
    }
    public virtual StubDispatchTarget fluent() {
        StubDispatchProbe.bodyCalls++;
        return this;
    }
    public virtual void write(List<SObject> values) {
        StubDispatchProbe.bodyCalls++;
        insert values;
    }
}
