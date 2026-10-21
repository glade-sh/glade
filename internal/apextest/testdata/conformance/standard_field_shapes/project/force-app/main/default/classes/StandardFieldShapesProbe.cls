public class StandardFieldShapesProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> observed = new Map<String,Object>();
        Map<String,Schema.SObjectType> types=Schema.getGlobalDescribe();
        for(String name:new List<String>{'ContactPointAddress','ContactPointEmail','QuickText','Account'}) {
            if(!types.containsKey(name)){observed.put(name,'unavailable');continue;}
            Map<String,Schema.SObjectField> fields=types.get(name).getDescribe().fields.getMap();
            for(String fieldName:new List<String>{'PreferenceRank','BestTimeToContactEndTime','Channel','NumberOfEmployees'}) {
                if(!fields.containsKey(fieldName))continue;
                Schema.DescribeFieldResult d=fields.get(fieldName).getDescribe();
                Map<String,Object> shape=new Map<String,Object>{'type'=>String.valueOf(d.getType()),'digits'=>d.getDigits(),'precision'=>d.getPrecision(),'scale'=>d.getScale()};
                List<String> values=new List<String>();
                for(Schema.PicklistEntry entry:d.getPicklistValues())if(entry.isActive())values.add(entry.getValue());
                shape.put('values',values); observed.put(name+'.'+fieldName,shape);
            }
        }
        return observed;
    }
}
