System.assertEquals('Site.getSiteId()', InertSiteLiteral.observe(), 'siteLiteral');
System.assertEquals('comment', InertSiteComment.observe(), 'siteComment');
System.assertEquals('SELECT IsPortalEnabled FROM User', InertCommunityQuery.observe(), 'communityQueryLiteral');
System.assertEquals('SELECT Id FROM Territory2', InertTerritoryQuery.observe(), 'territoryQueryLiteral');
System.assertEquals('comment', InertQueryComment.observe(), 'queryComment');
System.assertEquals('0/0', StaticAggregateAliases.observe(), 'staticAggregateAliases');
System.assertEquals('0/0', DynamicAggregateAliases.observe(), 'dynamicAggregateAliases');
System.assertEquals('false/false', SetupFieldMetadata.observe(), 'fieldMetadata');
