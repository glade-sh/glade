public abstract class PlanBase implements Writer {
    public enum Direction { ASCENDING, DESCENDING }
    public interface Writer { Writer orderBy(String field, PlanBase.Direction direction); Writer limitRows(Integer count); }
}
