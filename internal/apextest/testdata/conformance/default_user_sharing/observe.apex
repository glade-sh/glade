User owner=[SELECT Id FROM User WHERE LastName='ProbeSharingOwner' LIMIT 1];
ProbeItem__c item=new ProbeItem__c(Name='Original',OwnerId=owner.Id); insert item;
System.debug(LoggingLevel.ERROR,'CAP|'+ProbeSharing.observe(item.Id));
