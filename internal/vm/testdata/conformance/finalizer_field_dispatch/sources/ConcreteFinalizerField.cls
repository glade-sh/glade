public class ConcreteFinalizerField {
    public class Collector implements System.Finalizer {
        public Integer total=0;
        public void add(Object value) { total+=(Integer)value; }
        public void execute(System.FinalizerContext context) {}
    }
    public Collector finalizer=new Collector();
    public Collector supplied { get; set; }
    public Integer exercise() {
        this.finalizer.add(1);
        finalizer.add(2);
        Collector local=finalizer;
        local.add(4);
        supplied=local;
        this.supplied.add(8);
        return finalizer.total;
    }
    public static Map<String,Object> observe() {
        ConcreteFinalizerField holder=new ConcreteFinalizerField();
        Map<String,Object> rows=new Map<String,Object>{'sum'=>holder.exercise(),'inherited'=>new InheritedFinalizerWorker().exercise()};
        System.Finalizer callback=holder.finalizer;
        callback.execute(null);
        holder.finalizer=null;
        try { holder.finalizer.add(1); rows.put('null','accepted'); }
        catch(Exception e) { rows.put('null',e.getTypeName()+': '+e.getMessage()); }
        return rows;
    }
}
