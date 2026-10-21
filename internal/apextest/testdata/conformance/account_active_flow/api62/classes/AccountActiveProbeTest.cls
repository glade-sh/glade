@IsTest private class AccountActiveProbeTest { @IsTest static void observeActive() { System.assert(false, 'ACTIVE_RESULT|' + AccountActiveProbe.observe()); } }
