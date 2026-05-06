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
        'collections.map-remove-null' => 'CollectionProbes',
        // Async Apex
        'async.future-stub' => 'AsyncProbes',
        'async.queueable-stub' => 'AsyncProbes',
        'async.batchable-stub' => 'AsyncProbes',
        'async.schedulable-stub' => 'AsyncProbes',
        // Email & Messaging
        'email.single-message' => 'EmailProbes',
        'email.limits' => 'EmailProbes',
        // Integration
        'integration.http-request' => 'IntegrationProbes',
        'integration.json-serialize-complex' => 'IntegrationProbes',
        'integration.json-deserialize-untyped' => 'IntegrationProbes',
        'integration.encoding-util' => 'IntegrationProbes',
        // Metadata
        'metadata.custom-metadata-query' => 'MetadataProbes',
        'metadata.custom-setting-query' => 'MetadataProbes',
        'metadata.custom-metadata-describe' => 'MetadataProbes',
        // Platform Events
        'platform-event.publish' => 'PlatformEventProbes',
        'platform-event.describe' => 'PlatformEventProbes',
        // Schema Describe
        'schema.global-describe-size' => 'SchemaDescribeProbes',
        'schema.object-describe-fields' => 'SchemaDescribeProbes',
        'schema.picklist-describe' => 'SchemaDescribeProbes',
        'schema.record-type-describe' => 'SchemaDescribeProbes',
        // Security
        'security.user-info' => 'SecurityProbes',
        'security.profile-name' => 'SecurityProbes',
        'security.crud-check' => 'SecurityProbes',
        'security.fls-check' => 'SecurityProbes',
        // Bulk DML
        'bulkdml.partial-success' => 'BulkDmlProbes',
        'bulkdml.errors-shape' => 'BulkDmlProbes',
        'bulkdml.id-assignment' => 'BulkDmlProbes',
        // SObject & Id
        'sobject.clone' => 'SObjectProbes',
        'sobject.get-put' => 'SObjectProbes',
        'sobject.getSObjectType' => 'SObjectProbes',
        'id.validation' => 'SObjectProbes',
        // More Collections
        'collections.list-sort' => 'MoreCollectionProbes',
        'collections.map-keyset' => 'MoreCollectionProbes',
        'collections.set-equality' => 'MoreCollectionProbes',
        // More Datetime
        'datetime.now' => 'MoreDatetimeProbes',
        'datetime.today' => 'MoreDatetimeProbes',
        'datetime.valueOfGmt' => 'MoreDatetimeProbes',
        // More String
        'string.escapeSingleQuotes' => 'MoreStringProbes',
        'string.split' => 'MoreStringProbes',
        'string.trim' => 'MoreStringProbes',
        // More SOQL
        'soql.count-distinct' => 'MoreSoqlProbes',
        'soql.group-by' => 'MoreSoqlProbes',
        'soql.subquery' => 'MoreSoqlProbes',
        // System Assert
        'system.assert-pass' => 'SystemAssertProbes',
        'system.assertEquals' => 'SystemAssertProbes',
        // More Math
        'math.abs-negative' => 'MoreMathProbes',
        'math.max' => 'MoreMathProbes',
        'math.pow' => 'MoreMathProbes',
        'math.min' => 'MoreMathProbes',
        // More Integration
        'integration.json-serialize-sobject' => 'IntegrationMoreProbes',
        'integration.url-encoding' => 'IntegrationMoreProbes'
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

    public static String runMany(List<String> probeIds) {
        List<Object> results = new List<Object>();
        for (String probeId : probeIds) {
            results.add(JSON.deserializeUntyped(run(probeId)));
        }
        return JSON.serialize(results);
    }

    public static String runManyIsolated(List<String> probeIds) {
        List<Object> results = new List<Object>();
        for (String probeId : probeIds) {
            Savepoint sp = Database.setSavepoint();
            String raw = run(probeId);
            Database.rollback(sp);
            results.add(JSON.deserializeUntyped(raw));
        }
        return JSON.serialize(results);
    }

    public static String preflight() {
        Map<String, Object> shape = new Map<String, Object>();
        shape.put('organizationId', UserInfo.getOrganizationId());
        shape.put('userId', UserInfo.getUserId());
        shape.put('apiVersion', '60.0');
        shape.put('probeCount', PROBES.size());
        shape.put('hasProbeTestObject', Schema.getGlobalDescribe().containsKey('ProbeTestObject__c'));
        shape.put('hasProbeTestEvent', Schema.getGlobalDescribe().containsKey('ProbeTestEvent__e'));
        shape.put('hasProbeTestMdt', Schema.getGlobalDescribe().containsKey('ProbeTestMdt__mdt'));
        shape.put('hasProbeTestSetting', Schema.getGlobalDescribe().containsKey('ProbeTestSetting__c'));
        shape.put('probeTestObjectRows', [SELECT COUNT() FROM ProbeTestObject__c]);
        shape.put('isMultiCurrency', Schema.SObjectType.ProbeTestObject__c.fields.getMap().containsKey('CurrencyIsoCode'));
        return JSON.serialize(shape);
    }

    private static String resolveCategory(String className) {
        if (className.startsWith('StdlibString') || className.startsWith('StdlibDatetime') || className.startsWith('StdlibMath')) return 'Stdlib & System';
        if (className.startsWith('Soql') || className.startsWith('MoreSoql')) return 'Data Runtime';
        if (className.startsWith('DmlTrigger') || className.startsWith('BulkDml')) return 'DML & Triggers';
        if (className.startsWith('Limit')) return 'Limits & System';
        if (className.startsWith('Collection') || className.startsWith('MoreCollection')) return 'Collections & Language';
        if (className.startsWith('Async')) return 'Async Apex';
        if (className.startsWith('Email')) return 'Email & Messaging';
        if (className.startsWith('Integration')) return 'Integration';
        if (className.startsWith('Metadata')) return 'Metadata';
        if (className.startsWith('PlatformEvent')) return 'Platform Events';
        if (className.startsWith('SchemaDescribe')) return 'Schema Describe';
        if (className.startsWith('Security')) return 'Security & Sharing';
        if (className.startsWith('SObject')) return 'SObject & Type';
        if (className.startsWith('MoreDatetime')) return 'Stdlib & System';
        if (className.startsWith('MoreString')) return 'Stdlib & System';
        if (className.startsWith('MoreMath')) return 'Stdlib & System';
        if (className.startsWith('SystemAssert')) return 'System & Limits';
        return 'Uncategorized';
    }
}
