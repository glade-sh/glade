System.debug('GROUPS|' + JSON.serialize([SELECT Name, DeveloperName, Type FROM Group WHERE DeveloperName IN ('AllInternalUsers','AllCustomerPortalUsers','AllPartnerUsers') ORDER BY DeveloperName]));
