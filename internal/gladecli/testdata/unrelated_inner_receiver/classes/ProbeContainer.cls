public class ProbeContainer {
    public virtual class EventBus {
        public virtual Integer count(List<SObject> records) { return 41; }
    }
    public class Reader {
        public Integer own() { return new EventBus().count(new List<SObject>()); }
        public Integer explicitSystem() { return System.EventBus.publish(new List<SObject>()).size(); }
    }
}
