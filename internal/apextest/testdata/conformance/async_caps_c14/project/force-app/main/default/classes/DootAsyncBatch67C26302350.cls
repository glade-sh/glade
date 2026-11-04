public without sharing class DootAsyncBatch67C26302350 implements Database.Batchable<Integer> {
    private Id owned;
    public DootAsyncBatch67C26302350(Id owned) { this.owned=owned; }
    public Iterable<Integer> start(Database.BatchableContext context) { return new List<Integer>{1}; }
    public void execute(Database.BatchableContext context,List<Integer> scope) {
        Map<String,Object> data=DootAsyncWire67C26302350.getter('Y6.execute','Y6');
        data.put('scope',scope.size());
        DootAsyncWire67C26302350.persist(owned,data);
    }
    public void finish(Database.BatchableContext context) {}
}
