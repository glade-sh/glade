@IsTest public class NullRouteProbeTest {
    @IsTest static void typed() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('typed')); }
    @IsTest static void action() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('action')); }
    @IsTest static void subflow() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('subflow')); }
    @IsTest static void before_update() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('before_update')); }
    @IsTest static void before_upsert_insert() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('before_upsert_insert')); }
    @IsTest static void before_upsert_update() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('before_upsert_update')); }
    @IsTest static void after_insert() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('after_insert')); }
    @IsTest static void after_update() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('after_update')); }
    @IsTest static void after_upsert_insert() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('after_upsert_insert')); }
    @IsTest static void after_upsert_update() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('after_upsert_update')); }
    @IsTest static void before_delete() { throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.observe('before_delete')); }
    @IsTest static void asyncBoundary() { Test.startTest(); NullRouteProbe.enqueue(); Test.stopTest(); throw new NullRouteProbe.ObservationException('PROBE|' + NullRouteProbe.asyncRead()); }
}
