public with sharing class PickProbeController {
    public Account acct { get; set; }
    public Account blankAcct { get; set; }
    public PickProbe__c rec { get; set; }
    public PickProbe__c blankRec { get; set; }
    public PickProbe__c spaced { get; set; }
    public String str { get; set; }
    public String nullStr { get; set; }
    public PickProbeController() {
        acct = new Account(Name = 'Acme', Industry = 'Banking', Type = 'Customer - Direct');
        blankAcct = new Account(Name = 'Blank');
        rec = new PickProbe__c(Name = 'r', Stage__c = 'Open', Note__c = 'Open', Count__c = 1, Multi__c = 'A;B');
        blankRec = new PickProbe__c(Name = 'b');
        spaced = new PickProbe__c(Name = 's', Stage__c = 'On Hold');
        str = 'Open';
    }
    public PickProbeController(ApexPages.StandardController c) { this(); }
}
