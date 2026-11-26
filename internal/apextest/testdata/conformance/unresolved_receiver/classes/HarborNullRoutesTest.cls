@IsTest
private class HarborNullRoutesTest {
    @IsTest static void missingFieldToken() {
        Schema.SObjectField field = Schema.SObjectType.Account.fields.getMap().get('HarborUninstalledField__c');
        field.getDescribe();
    }

    @IsTest static void missingReflectiveType() {
        Type missing = Type.forName('HarborUninstalledReflectiveType');
        missing.newInstance();
    }

    @IsTest static void missingMapValue() {
        Map<String, Map<String, String>> maps = new Map<String, Map<String, String>>();
        maps.get('missing').get('value');
    }

    @IsTest static void presentFieldAndType() {
        Schema.SObjectField field = Schema.SObjectType.Account.fields.getMap().get('Name');
        System.assertEquals('Name', field.getDescribe().getName());
        Type present = Type.forName('Account');
        System.assert(present.newInstance() instanceof Account);
        Map<String, Map<String, String>> maps = new Map<String, Map<String, String>>{
            'present' => new Map<String, String>{'name' => 'Beacon'}
        };
        System.assertEquals('Beacon', maps.get('present').get('name'));
    }
}
