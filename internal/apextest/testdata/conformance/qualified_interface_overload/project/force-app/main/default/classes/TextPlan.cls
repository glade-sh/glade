public virtual class TextPlan extends PlanBase implements PlanBase.Writer {
    private String text = '';
    public TextPlan orderBy(Object field, PlanBase.Direction direction) {
        text = String.valueOf(field) + ':' + String.valueOf(direction); return this;
    }
    public TextPlan limitRows(Integer count) { text += ':' + count; return this; }
    public String render() { return text; }
}
