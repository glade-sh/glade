@IsTest private class BoundaryFlowProbeTest {
 @IsTest static void interview_direct() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('interview_direct'))); }
 @IsTest static void interview_subflow() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('interview_subflow'))); }
 @IsTest static void invocable_direct() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('invocable_direct'))); }
 @IsTest static void invocable_subflow() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('invocable_subflow'))); }
 @IsTest static void before_create() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('before_create'))); }
 @IsTest static void before_update() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('before_update'))); }
 @IsTest static void after_direct_create() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('after_direct_create'))); }
 @IsTest static void after_direct_update() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('after_direct_update'))); }
 @IsTest static void after_subflow_create() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('after_subflow_create'))); }
 @IsTest static void after_subflow_update() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('after_subflow_update'))); }
 @IsTest static void delete_direct() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('delete_direct'))); }
 @IsTest static void delete_subflow() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('delete_subflow'))); }
 @IsTest static void event_direct_explicit() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('event_direct_explicit'))); }
 @IsTest static void event_direct_stop() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('event_direct_stop'))); }
 @IsTest static void scheduled_dml() { System.assert(false,'OBS|'+JSON.serialize(BoundaryFlowProbe.observe('scheduled_dml'))); }
}
