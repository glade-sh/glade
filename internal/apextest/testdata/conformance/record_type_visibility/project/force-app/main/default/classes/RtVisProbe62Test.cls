@IsTest public class RtVisProbe62Test {
    @IsTest static void observeAccount() { emit('Account'); }
    @IsTest static void observeContact() { emit('Contact'); }
    @IsTest static void observeOpportunity() { emit('Opportunity'); }
    @IsTest static void observeNoRt() { emit('V35NoRt__c'); }
    @IsTest static void observeRt() { emit('V35Rt__c'); }
    static void emit(String obj) {
        Map<String,Object> all = RtVisProbe62.observe();
        Map<String,Object> m = new Map<String,Object>();
        for (String k : all.keySet()) { if (k.startsWith(obj + '.')) { m.put(k, all.get(k)); } }
        String text = JSON.serialize(m).replace(' If you are attempting to use a custom field, be sure to append the \'__c\' after the custom field name. Please reference your WSDL or the describe call for the appropriate names.', ' <hint_c>').replace(' If you are attempting to use a custom relationship, be sure to append the \'__r\' after the custom relationship name. Please reference your WSDL or the describe call for the appropriate names.', ' <hint_r>');
        System.assert(false, 'OBS|' + text);
    }
}
