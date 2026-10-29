@IsTest private class PersonAliasProbeTest { @IsTest static void observeAlias() { System.assert(false,'PERSON_ALIAS_RESULT|'+JSON.serialize(PersonAliasProbe.observe())); } }
