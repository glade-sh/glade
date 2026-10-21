@IsTest
private class PlatformConstructorSchemaTest {
    private static Map<String, Object> reflect(String name) {
        Map<String, Object> observation = new Map<String, Object>();
        try {
            Type token = Type.forName(name);
            observation.put('tokenName', token.getName());
            Object value = token.newInstance();
            observation.put('accepted', true);
            observation.put('objectName', ((SObject)value).getSObjectType().getDescribe().getName());
        } catch (Exception error) {
            observation.put('type', error.getTypeName());
            observation.put('message', error.getMessage());
        }
        return observation;
    }

    private static Map<String, Object> construct(Schema.SObjectType token) {
        Map<String, Object> observation = new Map<String, Object>();
        try {
            SObject value = token.newSObject();
            observation.put('accepted', true);
            observation.put('objectName', value.getSObjectType().getDescribe().getName());
        } catch (Exception error) {
            observation.put('type', error.getTypeName());
            observation.put('message', error.getMessage());
        }
        return observation;
    }

    private static void observe(String method, Map<String, Object> observation) {
        System.assert(false, 'OBSERVED|' + method + '|' + JSON.serialize(observation));
    }

    @IsTest static void profileConstruction() {
        observe('profileConstruction', new Map<String, Object>{
            'reflection' => reflect('Profile'),
            'schema' => construct(Profile.SObjectType),
            'createable' => Profile.SObjectType.getDescribe().isCreateable()
        });
    }

    @IsTest static void accountConstruction() {
        observe('accountConstruction', new Map<String, Object>{
            'reflection' => reflect('Account'),
            'schema' => construct(Account.SObjectType),
            'createable' => Account.SObjectType.getDescribe().isCreateable()
        });
    }

    @IsTest static void standardIntegerField() {
        Schema.DescribeFieldResult field = ApexClass.LengthWithoutComments.getDescribe();
        observe('standardIntegerField', new Map<String, Object>{
            'displayType' => String.valueOf(field.getType()),
            'soapType' => String.valueOf(field.getSOAPType()),
            'createable' => field.isCreateable(),
            'nillable' => field.isNillable()
        });
    }
}
