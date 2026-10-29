public class SObjectJSON2DecoderProbe {
    public static Map<String,Map<String,String>> observe() {
        Map<String,String> inputs = new Map<String,String>{
            'currencyNumber'=>'{"attributes":{"type":"Account"},"AnnualRevenue":33.335}',
            'currencyFormatted'=>'{"attributes":{"type":"Account"},"AnnualRevenue":"$33.34"}',
            'currencyNull'=>'{"attributes":{"type":"Account"},"AnnualRevenue":null}',
            'address'=>'{"attributes":{"type":"Account"},"BillingAddress":{"street":"1 Main","city":"Town","latitude":12.5,"longitude":-45.25}}',
            'addressEmpty'=>'{"attributes":{"type":"Account"},"BillingAddress":{}}',
            'location'=>'{"attributes":{"type":"Json_Place__c"},"Point__c":{"latitude":12.5,"longitude":-45.25}}',
            'locationEmpty'=>'{"attributes":{"type":"Json_Place__c"},"Point__c":{}}',
            'nullRecord'=>'null'
        };
        Map<String,Map<String,String>> rows = new Map<String,Map<String,String>>();
        for (String shape : inputs.keySet()) {
            Map<String,String> row = new Map<String,String>();
            for (String target : new List<String>{'sobject','concrete'}) {
                Type targetType = target == 'sobject' ? SObject.class : (shape.startsWith('location') ? Json_Place__c.class : Account.class);
                for (String route : new List<String>{'deserialize','strict','parser','parserStrict'}) {
                    try {
                        Object value;
                        if (route == 'deserialize') value = JSON.deserialize(inputs.get(shape),targetType);
                        else if (route == 'strict') value = JSON.deserializeStrict(inputs.get(shape),targetType);
                        else {
                            JSONParser parser = JSON.createParser(inputs.get(shape));
                            parser.nextToken();
                            value = route == 'parser' ? parser.readValueAs(targetType) : parser.readValueAsStrict(targetType);
                        }
                        row.put(target+'_'+route,'OK:'+JSON.serialize(value));
                    } catch (Exception problem) {
                        row.put(target+'_'+route,problem.getTypeName()+':'+problem.getMessage());
                    }
                }
            }
            rows.put(shape,row);
        }
        return rows;
    }
    public static void emit() {
        Map<String,Map<String,String>> rows = observe();
        for (String shape : rows.keySet()) System.debug(LoggingLevel.ERROR,'SOBJECT_JSON2_DECODER|'+shape+'|'+JSON.serialize(rows.get(shape)));
    }
}
