@IsTest public class RtMapProbe62Test {
    @IsTest static void observeAdmin() { emit('admin'); }
    @IsTest static void observeStandard() {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        User u = new User(Alias = 'rtstd', Email = 'rtstd62@example.com', EmailEncodingKey = 'UTF-8', LastName = 'RtStd', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', Username = 'rtstd62' + System.currentTimeMillis() + '@example.com');
        insert u;
        System.runAs(u) { emit('standard'); }
    }
    static void emit(String who) {
        String text = JSON.serialize(RtMapProbe62.observe());
        System.assert(false, 'OBS|' + who + '|' + text);
    }
}
