public class DescribeSharingProbe67 { public static void run() {
Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
System.debug('ROW\tpristine\t' + fields.get('name').getDescribe().getName() + ':' + fields.containsKey('NAME'));
try {
    fields.clear();
    System.debug('ROW\tclear\taccepted');
} catch (Exception e) {
    System.debug('ROW\tclear\t' + e.getTypeName() + ':' + e.getMessage());
}
System.debug('ROW\tafter\t' + Account.SObjectType.getDescribe().fields.getMap().get('name').getDescribe().getName());

} }
