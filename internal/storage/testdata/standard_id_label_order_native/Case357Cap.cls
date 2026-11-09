@IsTest
private class Case357Cap {
    @IsTest static void run() {
        for (String objectName : new List<String>{'Order', 'Account', 'Contact', 'Lead'}) {
            Schema.DescribeFieldResult f = Schema.getGlobalDescribe().get(objectName).getDescribe().fields.getMap().get('Id').getDescribe();
            System.debug('NATIVE_ROW:' + JSON.serialize(new Map<String, Object>{'route' => 'istest', 'object' => objectName, 'field' => 'Id', 'label' => f.getLabel(), 'direct' => 'getGlobalDescribe'}));
        }
        System.debug('NATIVE_ROW:' + JSON.serialize(new Map<String, Object>{'route' => 'istest', 'object' => 'Order', 'field' => 'Id', 'label' => Schema.SObjectType.Order.fields.Id.getLabel(), 'direct' => 'SObjectType.Order.fields.Id'}));
    }
}
