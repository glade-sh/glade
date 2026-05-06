public class ShapeProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'shape.exception-type-message') {
            return runExceptionTypeMessage();
        } else if (probeId == 'shape.soql-row-attributes') {
            return runSoqlRowAttributes();
        } else if (probeId == 'shape.json-sobject-attributes') {
            return runJsonSObjectAttributes();
        } else if (probeId == 'shape.describe-field-type-token') {
            return runDescribeFieldTypeToken();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runExceptionTypeMessage() {
        try {
            String value = null;
            value.length();
            return 'no-exception';
        } catch (Exception ex) {
            return ex.getTypeName() + '|' + String.valueOf(ex.getMessage() != null);
        }
    }

    private Object runSoqlRowAttributes() {
        ProbeTestObject__c row = [SELECT Id, Name__c FROM ProbeTestObject__c WHERE Name__c = 'Record1' LIMIT 1];
        String jsonText = JSON.serialize(row);
        return jsonText.contains('"attributes"') && jsonText.contains('"type":"ProbeTestObject__c"');
    }

    private Object runJsonSObjectAttributes() {
        ProbeTestObject__c row = new ProbeTestObject__c(Name__c = 'ShapeJson', Value__c = 9, CurrencyIsoCode = 'USD');
        String jsonText = JSON.serialize(row);
        return jsonText.contains('"attributes"') && jsonText.contains('"type":"ProbeTestObject__c"');
    }

    private Object runDescribeFieldTypeToken() {
        Schema.DescribeFieldResult fieldDescribe = ProbeTestObject__c.Value__c.getDescribe();
        return String.valueOf(fieldDescribe.getType());
    }
}
