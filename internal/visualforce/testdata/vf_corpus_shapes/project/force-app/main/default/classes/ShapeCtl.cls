public with sharing class ShapeCtl {
    public class Wrap { public String name { get; set; } public ApexPages.Action createNew { get { return new ApexPages.Action('{!doIt}'); } } }
    public Wrap wrap { get; set; }
    public String msg { get; set; }
    public List<Account> accts { get; set; }
    public List<String> objs { get; set; }
    public Map<String, List<String>> fieldMap { get; set; }
    public String extra { get; set; }
    public ShapeCtl() {
        wrap = new Wrap(); wrap.name = 'w'; msg = 'hi';
        accts = new List<Account>{ new Account(Name = 'A1', Description = 'd1'), new Account(Name = 'A2', Description = 'd2') };
        objs = new List<String>{'Account'};
        fieldMap = new Map<String, List<String>>{ 'Account' => new List<String>{'Name', 'Type'} };
    }
    public PageReference doIt() { msg = 'done'; return null; }
    public PageReference go() { msg = 'went'; return null; }
}