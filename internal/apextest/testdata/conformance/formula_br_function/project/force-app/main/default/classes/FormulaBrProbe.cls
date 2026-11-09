public class FormulaBrProbe {
    public static Map<String,Object> observe() {
        FormulaBrObj__c o = new FormulaBrObj__c(Name='n', Txt__c='banana');
        insert o;
        String f = [SELECT Fml__c FROM FormulaBrObj__c WHERE Id = :o.Id].Fml__c;
        return new Map<String,Object>{ 'value' => f, 'length' => f.length(), 'hasBrSlash' => f.contains('<br />'), 'hasBr' => f.contains('<br>') };
    }
}
