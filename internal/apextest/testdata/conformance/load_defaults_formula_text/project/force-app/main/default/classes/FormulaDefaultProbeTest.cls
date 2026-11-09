@IsTest public class FormulaDefaultProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(FormulaDefaultProbe.observe())); } }
