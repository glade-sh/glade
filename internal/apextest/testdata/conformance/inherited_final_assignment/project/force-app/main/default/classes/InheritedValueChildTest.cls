@IsTest public class InheritedValueChildTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(InheritedValueChild.observe())); } }
