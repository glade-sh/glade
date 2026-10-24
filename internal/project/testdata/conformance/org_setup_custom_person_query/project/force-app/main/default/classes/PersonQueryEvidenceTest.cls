@IsTest private class PersonQueryEvidenceTest {
    @IsTest static void staticSelect() { System.assertEquals('0', PersonQueryStaticSelect.observe()); }
    @IsTest static void staticWhere() { System.assertEquals('0', PersonQueryStaticWhere.observe()); }
    @IsTest static void staticOrder() { System.assertEquals('0', PersonQueryStaticOrder.observe()); }
    @IsTest static void staticAggregate() { System.assertEquals('0', PersonQueryStaticAggregate.observe()); }
    @IsTest static void staticQualified() { System.assertEquals('0', PersonQueryStaticQualified.observe()); }
    @IsTest static void query() { System.assertEquals('0', PersonQueryQuery.observe()); }
    @IsTest static void queryWithBinds() { System.assertEquals('0', PersonQueryQueryWithBinds.observe()); }
    @IsTest static void getQueryLocator() { System.assertEquals('false', PersonQueryGetQueryLocator.observe()); }
    @IsTest static void getQueryLocatorWithBinds() { System.assertEquals('false', PersonQueryGetQueryLocatorWithBinds.observe()); }
    @IsTest static void countQuery() { System.assertEquals('0', PersonQueryCountQuery.observe()); }
    @IsTest static void countQueryWithBinds() { System.assertEquals('0', PersonQueryCountQueryWithBinds.observe()); }
    @IsTest static void staticAlias() { System.assertEquals('0', PersonQueryStaticAlias.observe()); }
    @IsTest static void dynamicAlias() { System.assertEquals('0', PersonQueryDynamicAlias.observe()); }
    @IsTest static void contactField() { System.assertEquals('0', PersonQueryContactField.observe()); }
    @IsTest static void missingField() { System.assertEquals('missing', PersonQueryMissingField.observe()); }
    @IsTest static void inertLiteral() { System.assertEquals('inert', PersonQueryInertLiteral.observe()); }
    @IsTest static void fieldMetadata() { System.assertEquals('true/true', PersonQueryFieldMetadata.observe()); }
}
