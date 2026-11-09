public class RtEnum {
    public static void run() {
        for (String obj : new List<String>{'Order','OrderItem'}) {
            Map<String,Schema.SObjectField> m = Schema.getGlobalDescribe().get(obj).getDescribe().fields.getMap();
            List<String> names = new List<String>(m.keySet()); names.sort();
            for (String n : names) { Schema.DescribeFieldResult d = m.get(n).getDescribe();
                System.debug('F|' + obj + '|' + d.getName() + '|' + d.getType() + '|a' + d.isAccessible() + '|c' + d.isCreateable() + '|u' + d.isUpdateable() + '|n' + d.isNillable() + '|p' + d.isPermissionable() + '|len' + d.getLength() + '|pr' + d.getPrecision() + '|s' + d.getScale() + '|' + d.getLabel()); }
        }
    }
}
