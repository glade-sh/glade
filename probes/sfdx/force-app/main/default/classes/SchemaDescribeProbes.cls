public class SchemaDescribeProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'schema.global-describe-size') {
            return runGlobalDescribeSize();
        } else if (probeId == 'schema.object-describe-fields') {
            return runObjectDescribeFields();
        } else if (probeId == 'schema.picklist-describe') {
            return runPicklistDescribe();
        } else if (probeId == 'schema.record-type-describe') {
            return runRecordTypeDescribe();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runGlobalDescribeSize() {
        Map<String, Schema.SObjectType> gd = Schema.getGlobalDescribe();
        return gd.size() > 0;
    }

    private Object runObjectDescribeFields() {
        Schema.DescribeSObjectResult d = Account.SObjectType.getDescribe();
        return d.fields.getMap().size() > 0;
    }

    private Object runPicklistDescribe() {
        Schema.DescribeFieldResult d = Account.Industry.getDescribe();
        return String.valueOf(d.getType());
    }

    private Object runRecordTypeDescribe() {
        Schema.DescribeSObjectResult d = Account.SObjectType.getDescribe();
        return d.getRecordTypeInfos().size() > 0;
    }
}
