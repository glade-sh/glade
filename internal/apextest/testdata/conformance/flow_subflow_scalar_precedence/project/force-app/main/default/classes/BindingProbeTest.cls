@IsTest private class BindingProbeTest { @IsTest static void observeScalarPrecedence() { System.assert(false,'OBS|'+JSON.serialize(BindingProbe.observe())); } }
