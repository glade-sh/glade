@IsTest private class SetupEvidenceTest {
    @IsTest static void siteLiteral() { System.assertEquals('Site.getSiteId()', InertSiteLiteral.observe()); }
    @IsTest static void siteComment() { System.assertEquals('comment', InertSiteComment.observe()); }
    @IsTest static void communityQueryLiteral() { System.assertEquals('SELECT IsPortalEnabled FROM User', InertCommunityQuery.observe()); }
    @IsTest static void territoryQueryLiteral() { System.assertEquals('SELECT Id FROM Territory2', InertTerritoryQuery.observe()); }
    @IsTest static void queryComment() { System.assertEquals('comment', InertQueryComment.observe()); }
    @IsTest static void staticAggregateAliases() { System.assertEquals('0/0', StaticAggregateAliases.observe()); }
    @IsTest static void dynamicAggregateAliases() { System.assertEquals('0/0', DynamicAggregateAliases.observe()); }
    @IsTest static void fieldMetadata() { System.assertEquals('false/false', SetupFieldMetadata.observe()); }
}
