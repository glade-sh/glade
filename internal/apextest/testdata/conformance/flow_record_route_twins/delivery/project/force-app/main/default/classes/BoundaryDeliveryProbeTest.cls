@IsTest private class BoundaryDeliveryProbeTest {
 @IsTest static void event_explicit() { System.assert(false,'OBS|'+JSON.serialize(BoundaryDeliveryProbe.eventDelivery('explicit'))); }
 @IsTest static void event_stop() { System.assert(false,'OBS|'+JSON.serialize(BoundaryDeliveryProbe.eventDelivery('stop'))); }
 @IsTest static void scheduled_stop() { System.assert(false,'OBS|'+JSON.serialize(BoundaryDeliveryProbe.scheduledDelivery('stop'))); }
 @IsTest static void scheduled_direct() { System.assert(false,'OBS|'+JSON.serialize(BoundaryDeliveryProbe.scheduledDelivery('direct'))); }
 @IsTest static void scheduled_subflow() { System.assert(false,'OBS|'+JSON.serialize(BoundaryDeliveryProbe.scheduledDelivery('subflow'))); }
 @IsTest static void scheduled_direct_static() { System.assert(false,'OBS|'+JSON.serialize(BoundaryDeliveryProbe.scheduledDelivery('direct_static'))); }
 @IsTest static void scheduled_subflow_static() { System.assert(false,'OBS|'+JSON.serialize(BoundaryDeliveryProbe.scheduledDelivery('subflow_static'))); }
 @IsTest static void scheduled_direct_namespace() { System.assert(false,'OBS|'+JSON.serialize(BoundaryDeliveryProbe.scheduledDelivery('direct_namespace'))); }
 @IsTest static void scheduled_subflow_namespace() { System.assert(false,'OBS|'+JSON.serialize(BoundaryDeliveryProbe.scheduledDelivery('subflow_namespace'))); }
}
