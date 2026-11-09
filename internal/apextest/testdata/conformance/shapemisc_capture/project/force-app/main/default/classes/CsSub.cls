public class CsSub extends CsHolder {
    public override String readImplicit() { return String.valueOf(lst.Text__c); }
    public String readSuper() { return super.readImplicit(); }
}
