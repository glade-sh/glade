public class C11Seed {
    public static void run() {
        insert new C11Cfg__c(Name = 'seed', RtName__c = 'Alpha', Flag__c = 'on');
        insert new C11Hier__c(Flag__c = 'hier-on');
        insert new C11Rec__c(Name = 'preexisting', Kind__c = 'seed');
    }
}
