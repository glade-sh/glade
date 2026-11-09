List<ProductCatalog> l = [SELECT Id FROM ProductCatalog WHERE Name = 'x']; System.debug('OBS|b27|' + String.valueOf(l.size()));
