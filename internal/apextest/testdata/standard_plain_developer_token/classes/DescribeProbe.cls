public class DescribeProbe {
    public static Schema.SObjectField token() { return Account.SObjectType.getDescribe().fields.getMap().get('Tier'); }
}
