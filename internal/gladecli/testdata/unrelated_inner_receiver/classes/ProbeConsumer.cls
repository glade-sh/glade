public class ProbeConsumer {
    public virtual class Request {
        public virtual Integer run() { return EventBus.publish(new List<SObject>()).size(); }
        public Integer explicitSystem() { return System.EventBus.publish(new List<SObject>()).size(); }
        public Integer qualified() { return new ProbeContainer.EventBus().count(new List<SObject>()); }
    }
}
