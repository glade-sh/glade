public class AcctHolder {
    public Account acct;
    public String readThis() { return this.acct.Name; }
    public String readImplicit() { return acct.Name; }
}
