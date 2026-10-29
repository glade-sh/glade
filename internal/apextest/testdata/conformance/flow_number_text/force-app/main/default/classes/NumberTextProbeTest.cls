@IsTest private class NumberTextProbeTest { @IsTest static void observeNumber() { System.assert(false,'NUMBER_TEXT_RESULT|'+JSON.serialize(NumberTextProbe.observe())); } }
