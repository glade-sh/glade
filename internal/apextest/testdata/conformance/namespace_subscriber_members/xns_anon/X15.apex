gladens004__V035Env__c e = new gladens004__V035Env__c(Name='n'); insert e; System.debug('OBS|X15|' + new ToneRelay(new QuietTone()).relayEnvelope(e));
