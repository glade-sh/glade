public class fflib_Noise {
    public static void frameworkDescribeWork(Schema.SObjectType sObjectType) {
        String typeName = sObjectType.getDescribe().getName();
        Map<String, Schema.SObjectField> fieldsByName = sObjectType.getDescribe().fields.getMap();
    }
}
