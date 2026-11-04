public without sharing class DootAsyncQueue67C26302350 implements Queueable {
    private Id owned;
    public DootAsyncQueue67C26302350(Id owned) { this.owned=owned; }
    public void execute(QueueableContext context) {
        Map<String,Object> data=DootAsyncWire67C26302350.relationship('L1.execute');
        DootAsyncWire67C26302350.persist(owned,data);
    }
}
