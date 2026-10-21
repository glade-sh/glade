public class ObjectAccessProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> observed = new Map<String,Object>();
        Map<String,Schema.SObjectType> types = Schema.getGlobalDescribe();
        for (String name : new List<String>{'Account','Organization','Profile','RecordType'}) {
            Schema.DescribeSObjectResult info = types.get(name).getDescribe();
            observed.put(name, new List<Boolean>{info.isCreateable(),info.isUpdateable(),info.isDeletable()});
        }
        return observed;
    }
}
