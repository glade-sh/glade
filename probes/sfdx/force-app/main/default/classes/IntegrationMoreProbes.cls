public class IntegrationMoreProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'integration.json-serialize-sobject') {
            return runJsonSerializeSobject();
        } else if (probeId == 'integration.url-encoding') {
            return runUrlEncoding();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runJsonSerializeSobject() {
        Account acc = new Account(Name='TestAccount');
        String jsonStr = JSON.serialize(acc);
        return jsonStr.contains('TestAccount');
    }

    private Object runUrlEncoding() {
        return EncodingUtil.urlEncode('hello world', 'UTF-8');
    }
}
