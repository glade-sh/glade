PaymentGatewayProvider p = [SELECT ApexAdapterId FROM PaymentGatewayProvider WHERE DeveloperName = 'ProbeInherited62' LIMIT 1];
ApexClass c = [SELECT Id FROM ApexClass WHERE Name = 'ProbeInherited62' LIMIT 1];
System.debug('PROVIDER_ADAPTER_EQUALS_CLASS=' + (p.ApexAdapterId == c.Id));
System.debug('PROVIDER_ADAPTER_DIFFERS_OTHER=' + (p.ApexAdapterId != [SELECT Id FROM ApexClass WHERE Name = 'ProbeInherited67' LIMIT 1].Id));
