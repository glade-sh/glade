for (PaymentGatewayProvider p : [SELECT DeveloperName,MasterLabel,Comments,IdempotencySupported,NamespacePrefix,ApexAdapterId FROM PaymentGatewayProvider WHERE DeveloperName='ProbeGateway']) {
 System.debug('PG_ROW='+p.DeveloperName+'|'+p.MasterLabel+'|'+p.Comments+'|'+p.IdempotencySupported+'|'+p.NamespacePrefix+'|'+(p.ApexAdapterId != null));
}
System.debug('PG_MISSING='+[SELECT count() FROM PaymentGatewayProvider WHERE DeveloperName='ProbeMissing']);
