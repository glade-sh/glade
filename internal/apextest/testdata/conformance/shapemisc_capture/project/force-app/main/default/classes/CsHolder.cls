public virtual class CsHolder {
    public ProbeList__c lst;
    public ProbeHier__c hier;
    public static ProbeList__c slst;
    public CsHolder nested;
    public virtual String readImplicit() { return String.valueOf(lst.Text__c); }
    public virtual String readThis() { return String.valueOf(this.lst.Text__c); }
    public virtual String readHier() { return String.valueOf(hier.Text__c); }
    public String readThisHier() { return String.valueOf(this.hier.Text__c); }
    public String readThisId() { return String.valueOf(this.lst.Id); }
    public String readThisHierId() { return String.valueOf(this.hier.Id); }
    public String readThisName() { return String.valueOf(this.lst.Name); }
    public String readSelf() { CsHolder self = this; return String.valueOf(self.lst.Text__c); }
    public ProbeList__c getLst() { return lst; }
    public static String readStatic() { return String.valueOf(slst.Text__c); }
    public static String readParam(CsHolder p) { return String.valueOf(p.lst.Text__c); }
}
