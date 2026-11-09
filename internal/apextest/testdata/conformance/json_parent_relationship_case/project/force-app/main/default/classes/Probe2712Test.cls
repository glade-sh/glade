@IsTest private class Probe2712Test {
 @IsTest static void observe() { System.assert(false,'OBSERVED|observe|'+JSON.serialize(Probe2712.observe('observe'))); }
}
