public class FormulaFailureProbe {
    public static String sample() {
        Map<String, Object> observed = new Map<String, Object>();
        try { throw new FormulaEvaluationException('synthetic expression'); }
        catch (FormulaEvaluationException problem) {
            Exception parent = problem;
            observed.put('message', parent.getMessage());
            observed.put('type', problem.getTypeName());
            observed.put('caught', true);
        }
        try { throw new FormulaEvaluationException('parent catch'); }
        catch (Exception problem) { observed.put('parentCaught', problem.getMessage()); }
        return JSON.serialize(observed);
    }
}
