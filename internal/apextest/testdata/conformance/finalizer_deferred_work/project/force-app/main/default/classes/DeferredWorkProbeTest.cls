@IsTest private class DeferredWorkProbeTest {
@IsTest static void finalizerChild() { System.assert(false, 'OBSERVED|finalizerChild|' + JSON.serialize(DeferredWorkProbe.finalizerChild())); }
@IsTest static void bodyChild() { System.assert(false, 'OBSERVED|bodyChild|' + JSON.serialize(DeferredWorkProbe.bodyChild())); }
@IsTest static void noChild() { System.assert(false, 'OBSERVED|noChild|' + JSON.serialize(DeferredWorkProbe.noChild())); }
 @IsTest static void bodyChildCapped() { System.assert(false,'OBSERVED|bodyChildCapped|'+JSON.serialize(DeferredWorkProbe.bodyChildCapped())); }
 @IsTest static void bodyChildDepthOne() { System.assert(false,'OBSERVED|bodyChildDepthOne|'+JSON.serialize(DeferredWorkProbe.bodyChildDepthOne())); }
 @IsTest static void bodyTwoChildren() { System.assert(false,'OBSERVED|bodyTwoChildren|'+JSON.serialize(DeferredWorkProbe.bodyTwoChildren())); }
 @IsTest static void bodyChildDelay() { System.assert(false,'OBSERVED|bodyChildDelay|'+JSON.serialize(DeferredWorkProbe.bodyChildDelay())); }
 @IsTest static void bodyChildOptions() { System.assert(false,'OBSERVED|bodyChildOptions|'+JSON.serialize(DeferredWorkProbe.bodyChildOptions())); }
 @IsTest static void finalizerChildDelay() { System.assert(false,'OBSERVED|finalizerChildDelay|'+JSON.serialize(DeferredWorkProbe.finalizerChildDelay())); }
 @IsTest static void finalizerChildOptions() { System.assert(false,'OBSERVED|finalizerChildOptions|'+JSON.serialize(DeferredWorkProbe.finalizerChildOptions())); }
}
