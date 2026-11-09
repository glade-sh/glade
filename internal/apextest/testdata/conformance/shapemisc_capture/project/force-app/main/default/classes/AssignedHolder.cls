public class AssignedHolder {
    public ProbeList__c lst = ProbeList__c.getValues('absent');
    public String read() { return String.valueOf(this.lst.Text__c); }
}
