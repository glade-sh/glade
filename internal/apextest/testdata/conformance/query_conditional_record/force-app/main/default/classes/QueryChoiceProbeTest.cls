@IsTest private class QueryChoiceProbeTest { @IsTest static void observeChoice() { System.assert(false,'QUERY_CHOICE_RESULT|'+JSON.serialize(QueryChoiceProbe.observe())); } }
