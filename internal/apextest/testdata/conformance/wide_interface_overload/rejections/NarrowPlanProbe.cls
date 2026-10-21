public class NarrowPlanProbe { public static TextPlan run(PlanBase.Writer writer) { return writer.orderBy('Name', PlanBase.Direction.ASCENDING); } }
