global class StubProbe {
    global static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        if (!Test.isRunningTest()) { result.put('context', 'anonymous'); return result; }
        StubRecorder.seen = new List<String>();
        StubService service = new StubService();
        service.domain = (StubDomain) Test.createStub(StubDomain.class, new StubRecorder());
        service.run(new List<Thing__c>());
        result.put('stubCalls', StubRecorder.seen);
        Type literal = List<Thing__c>.class;
        result.put('literalName', literal.getName());
        result.put('literalValueOf', String.valueOf(literal));
        result.put('directEq', literal == List<Thing__c>.class);
        result.put('forNameShortEq', Type.forName('List<Thing__c>') == literal);
        result.put('forNameFullEq', Type.forName('List<gladens004__Thing__c>') == literal);
        result.put('forNameShortNull', Type.forName('List<Thing__c>') == null);
        result.put('forNameFullNull', Type.forName('List<gladens004__Thing__c>') == null);
        result.put('elementEq', Type.forName('gladens004__Thing__c') == Thing__c.class);
        result.put('elementShortEq', Type.forName('Thing__c') == Thing__c.class);
        result.put('elementShortNull', Type.forName('Thing__c') == null);
        result.put('thingName', Thing__c.class.getName());
        result.put('setEq', Set<Thing__c>.class == Set<Thing__c>.class);
        result.put('mapKeyEq', new Map<Type, String>{ List<Thing__c>.class => 'x' }.containsKey(List<Thing__c>.class));
        return result;
    }
}
