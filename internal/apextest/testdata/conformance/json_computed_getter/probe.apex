Map<String, Object> rows = new Map<String, Object>();
Map<String, String> inputs = new Map<String, String>{
    'absent' => '{"seed":7}',
    'valid_computed' => '{"seed":7,"computed":99}',
    'invalid_computed' => '{"seed":7,"computed":{"nested":99}}',
    'null_computed' => '{"seed":7,"computed":null}',
    'folded_computed' => '{"seed":7,"COMPUTED":99}',
    'fallback' => '{"fallback":99}',
    'auto_read' => '{"autoRead":9}',
    'private_write' => '{"privateWrite":10}',
    'writable' => '{"writable":"opal"}',
    'unknown' => '{"missing":9}'
};
for (String key : inputs.keySet()) {
    for (Boolean strict : new List<Boolean>{false, true}) {
        String name = key + (strict ? '_strict' : '_lax');
        try {
            QuartzPayload decoded = (QuartzPayload)(strict
                ? JSON.deserializeStrict(inputs.get(key), QuartzPayload.class)
                : JSON.deserialize(inputs.get(key), QuartzPayload.class));
            rows.put(name, new List<Object>{decoded.seed, decoded.computed, decoded.autoRead, decoded.fallback,
                decoded.privateWrite, decoded.writable, decoded.readComputed(), JSON.deserializeUntyped(JSON.serialize(decoded))});
        } catch (Exception e) {
            rows.put(name, new List<String>{e.getTypeName(), e.getMessage()});
        }
    }
}
