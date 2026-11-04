public without sharing class DootAsyncSched67C26302350 implements Schedulable {
    private Id owned;
    public DootAsyncSched67C26302350(Id owned) { this.owned=owned; }
    public void execute(SchedulableContext context) {
        Map<String,Object> data=DootAsyncWire67C26302350.getter('Y7.execute','Y7');
        DootAsyncWire67C26302350.persist(owned,data);
    }
}
