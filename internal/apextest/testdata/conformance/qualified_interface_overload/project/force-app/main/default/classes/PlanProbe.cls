public class PlanProbe {
    public static String concrete() {
        TextPlan plan = new TextPlan().orderBy('Name', PlanBase.Direction.DESCENDING).limitRows(4);
        return plan.render();
    }
    public static String inherited() {
        TextPlan plan = new ChildPlan().orderBy('Id', PlanBase.Direction.ASCENDING).limitRows(2);
        return plan.render();
    }
    public static String throughInterface() {
        PlanBase.Writer writer = new TextPlan();
        PlanBase.Writer result = writer.orderBy('Name', PlanBase.Direction.ASCENDING);
        return ((TextPlan)result).render();
    }
}
