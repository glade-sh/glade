public class IntegrationProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'integration.http-request') {
            return runHttpRequest();
        } else if (probeId == 'integration.json-serialize-complex') {
            return runJsonSerializeComplex();
        } else if (probeId == 'integration.json-deserialize-untyped') {
            return runJsonDeserializeUntyped();
        } else if (probeId == 'integration.encoding-util') {
            return runEncodingUtil();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runHttpRequest() {
        HttpRequest req = new HttpRequest();
        req.setEndpoint('https://example.com');
        req.setMethod('GET');
        return req.getMethod();
    }

    private Object runJsonSerializeComplex() {
        Map<String, Object> m = new Map<String, Object>();
        m.put('a', 1);
        m.put('b', 'test');
        m.put('c', null);
        Map<String, Object> roundTrip = (Map<String, Object>) JSON.deserializeUntyped(JSON.serialize(m));
        return roundTrip.get('b');
    }

    private Object runJsonDeserializeUntyped() {
        String jsonStr = '{"a":1,"b":"test"}';
        Object o = JSON.deserializeUntyped(jsonStr);
        Map<String, Object> m = (Map<String, Object>) o;
        return m.get('b');
    }

    private Object runEncodingUtil() {
        String encoded = EncodingUtil.base64Encode(Blob.valueOf('hello'));
        return encoded;
    }
}
