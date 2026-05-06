public class ProbeRunner {
    public static final Map<String, String> PROBES = new Map<String, String>{
        // Stdlib & System
        'stdlib.string.format-null' => 'StdlibStringProbes',
        'stdlib.string.join-empty' => 'StdlibStringProbes',
        'stdlib.string.containsIgnoreCase-null' => 'StdlibStringProbes',
        'stdlib.string.valueOf-null' => 'StdlibStringProbes',
        'stdlib.string.isBlank-whitespace' => 'StdlibStringProbes',
        'stdlib.datetime.valueOf-null' => 'StdlibDatetimeProbes',
        'stdlib.datetime.leapYear' => 'StdlibDatetimeProbes',
        'stdlib.datetime.format-timezone' => 'StdlibDatetimeProbes',
        'stdlib.datetime.valueOf-invalid' => 'StdlibDatetimeProbes',
        'stdlib.datetime.yearZero' => 'StdlibDatetimeProbes',
        'stdlib.math.divide-scale' => 'StdlibMathProbes',
        'stdlib.math.mod-negative' => 'StdlibMathProbes',
        'stdlib.math.round-halfUp' => 'StdlibMathProbes',
        'stdlib.math.decimalValueOf-null' => 'StdlibMathProbes',
        'stdlib.math.log10' => 'StdlibMathProbes',
        // SOQL / Data Runtime
        'soql.select-all' => 'SoqlProbes',
        'soql.aggregate-count' => 'SoqlProbes',
        'soql.where-like' => 'SoqlProbes',
        'soql.order-desc' => 'SoqlProbes',
        'soql.dynamic' => 'SoqlProbes',
        // DML & Triggers
        'dml.insert-trigger' => 'DmlTriggerProbes',
        'dml.update-return' => 'DmlTriggerProbes',
        'dml.delete-return' => 'DmlTriggerProbes',
        'dml.undelete' => 'DmlTriggerProbes',
        'dml.insert-fail-duplicate' => 'DmlTriggerProbes',
        // Limits & System
        'limits.soql-before-after' => 'LimitProbes',
        'limits.dml-rows' => 'LimitProbes',
        'limits.heap-size' => 'LimitProbes',
        'limits.limit-queries' => 'LimitProbes',
        'limits.cpu-time' => 'LimitProbes',
        // Collections / Language
        'collections.list-contains-null' => 'CollectionProbes',
        'collections.map-null-key' => 'CollectionProbes',
        'collections.set-contains-null' => 'CollectionProbes',
        'collections.list-indexof-null' => 'CollectionProbes',
        'collections.map-remove-null' => 'CollectionProbes'
    };

    public static String run(String probeId) {
        String className = PROBES.get(probeId);
        if (className == null) {
            return JSON.serialize(new Map<String, Object>{
                'probeId' => probeId,
                'category' => 'unknown',
                'result' => null,
                'exceptionType' => 'UnknownProbeException',
                'exceptionMessage' => 'No probe registered for: ' + probeId
            });
        }

        String category = resolveCategory(className);
        Map<String, Object> payload = new Map<String, Object>{
            'probeId' => probeId,
            'category' => category
        };

        try {
            Type t = Type.forName(className);
            Probe p = (Probe) t.newInstance();
            Object result = p.run(probeId);
            payload.put('result', result);
            payload.put('exceptionType', null);
            payload.put('exceptionMessage', null);
        } catch (Exception ex) {
            payload.put('result', null);
            payload.put('exceptionType', ex.getTypeName());
            payload.put('exceptionMessage', ex.getMessage());
        }

        return JSON.serialize(payload);
    }

    private static String resolveCategory(String className) {
        if (className.startsWith('StdlibString') || className.startsWith('StdlibDatetime') || className.startsWith('StdlibMath')) return 'Stdlib & System';
        if (className.startsWith('Soql')) return 'Data Runtime';
        if (className.startsWith('DmlTrigger')) return 'DML & Triggers';
        if (className.startsWith('Limit')) return 'Limits & System';
        if (className.startsWith('Collection')) return 'Collections & Language';
        return 'Uncategorized';
    }
}
