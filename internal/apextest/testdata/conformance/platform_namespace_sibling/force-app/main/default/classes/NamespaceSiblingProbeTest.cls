@IsTest private class NamespaceSiblingProbeTest { @IsTest static void observeSibling() { System.assert(false,'NAMESPACE_SIBLING_RESULT|'+JSON.serialize(NamespaceSiblingProbe.observe())); } }
