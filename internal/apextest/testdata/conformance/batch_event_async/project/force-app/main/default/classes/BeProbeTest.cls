@IsTest public class BeProbeTest {
    @IsTest static void b_start_pe_none() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pe', 'none', false))); }
    @IsTest static void b_start_pe_future() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pe', 'future', false))); }
    @IsTest static void b_start_pe_batch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pe', 'batch', false))); }
    @IsTest static void b_start_pe_schedBatch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pe', 'schedBatch', false))); }
    @IsTest static void b_start_pe_queue() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pe', 'queue', false))); }
    @IsTest static void b_start_pe_schedule() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pe', 'schedule', false))); }
    @IsTest static void b_start_pei_none() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pei', 'none', false))); }
    @IsTest static void b_start_pei_future() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pei', 'future', false))); }
    @IsTest static void b_start_pei_batch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pei', 'batch', false))); }
    @IsTest static void b_start_pei_schedBatch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pei', 'schedBatch', false))); }
    @IsTest static void b_start_pei_queue() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pei', 'queue', false))); }
    @IsTest static void b_start_pei_schedule() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'pei', 'schedule', false))); }
    @IsTest static void b_start_cdc_none() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'cdc', 'none', false))); }
    @IsTest static void b_start_cdc_future() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'cdc', 'future', false))); }
    @IsTest static void b_start_cdc_batch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'cdc', 'batch', false))); }
    @IsTest static void b_start_cdc_schedBatch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'cdc', 'schedBatch', false))); }
    @IsTest static void b_start_cdc_queue() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'cdc', 'queue', false))); }
    @IsTest static void b_start_cdc_schedule() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('start', 'cdc', 'schedule', false))); }
    @IsTest static void b_execute_pe_none() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'none', false))); }
    @IsTest static void b_execute_pe_none_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'none', true))); }
    @IsTest static void b_execute_pe_future() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'future', false))); }
    @IsTest static void b_execute_pe_future_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'future', true))); }
    @IsTest static void b_execute_pe_batch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'batch', false))); }
    @IsTest static void b_execute_pe_batch_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'batch', true))); }
    @IsTest static void b_execute_pe_schedBatch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'schedBatch', false))); }
    @IsTest static void b_execute_pe_schedBatch_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'schedBatch', true))); }
    @IsTest static void b_execute_pe_queue() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'queue', false))); }
    @IsTest static void b_execute_pe_queue_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'queue', true))); }
    @IsTest static void b_execute_pe_schedule() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'schedule', false))); }
    @IsTest static void b_execute_pe_schedule_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pe', 'schedule', true))); }
    @IsTest static void b_execute_pei_none() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'none', false))); }
    @IsTest static void b_execute_pei_none_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'none', true))); }
    @IsTest static void b_execute_pei_future() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'future', false))); }
    @IsTest static void b_execute_pei_future_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'future', true))); }
    @IsTest static void b_execute_pei_batch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'batch', false))); }
    @IsTest static void b_execute_pei_batch_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'batch', true))); }
    @IsTest static void b_execute_pei_schedBatch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'schedBatch', false))); }
    @IsTest static void b_execute_pei_schedBatch_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'schedBatch', true))); }
    @IsTest static void b_execute_pei_queue() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'queue', false))); }
    @IsTest static void b_execute_pei_queue_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'queue', true))); }
    @IsTest static void b_execute_pei_schedule() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'schedule', false))); }
    @IsTest static void b_execute_pei_schedule_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'pei', 'schedule', true))); }
    @IsTest static void b_execute_cdc_none() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'none', false))); }
    @IsTest static void b_execute_cdc_none_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'none', true))); }
    @IsTest static void b_execute_cdc_future() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'future', false))); }
    @IsTest static void b_execute_cdc_future_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'future', true))); }
    @IsTest static void b_execute_cdc_batch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'batch', false))); }
    @IsTest static void b_execute_cdc_batch_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'batch', true))); }
    @IsTest static void b_execute_cdc_schedBatch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'schedBatch', false))); }
    @IsTest static void b_execute_cdc_schedBatch_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'schedBatch', true))); }
    @IsTest static void b_execute_cdc_queue() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'queue', false))); }
    @IsTest static void b_execute_cdc_queue_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'queue', true))); }
    @IsTest static void b_execute_cdc_schedule() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'schedule', false))); }
    @IsTest static void b_execute_cdc_schedule_dlv() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runBatch('execute', 'cdc', 'schedule', true))); }
    @IsTest static void d_pe_none() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('pe', 'none'))); }
    @IsTest static void d_pe_future() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('pe', 'future'))); }
    @IsTest static void d_pe_batch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('pe', 'batch'))); }
    @IsTest static void d_pe_schedBatch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('pe', 'schedBatch'))); }
    @IsTest static void d_pe_queue() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('pe', 'queue'))); }
    @IsTest static void d_pe_schedule() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('pe', 'schedule'))); }
    @IsTest static void d_cdc_none() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('cdc', 'none'))); }
    @IsTest static void d_cdc_future() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('cdc', 'future'))); }
    @IsTest static void d_cdc_batch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('cdc', 'batch'))); }
    @IsTest static void d_cdc_schedBatch() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('cdc', 'schedBatch'))); }
    @IsTest static void d_cdc_queue() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('cdc', 'queue'))); }
    @IsTest static void d_cdc_schedule() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.runDirect('cdc', 'schedule'))); }
    @IsTest static void two_inside() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.twoBatches('inside'))); }
    @IsTest static void two_beforeAndInside() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.twoBatches('beforeAndInside'))); }
    @IsTest static void two_insideThenAfter() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.twoBatches('insideThenAfter'))); }
    @IsTest static void two_beforeOnly() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.twoBatches('beforeOnly'))); }
    @IsTest static void two_noStart() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.twoBatches('noStart'))); }
    @IsTest static void skip1() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.skip(1))); }
    @IsTest static void skip2() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.skip(2))); }
    @IsTest static void skip3() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.skip(3))); }
    @IsTest static void skip4() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.skip(4))); }
    @IsTest static void skip5() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.skip(5))); }
    @IsTest static void throw_pe_deliver() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.throwing('pe', true))); }
    @IsTest static void throw_pe_stop() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.throwing('pe', false))); }
    @IsTest static void throw_cdc_deliver() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.throwing('cdc', true))); }
    @IsTest static void throw_cdc_stop() { System.assert(false, 'OBS|' + JSON.serialize(BeProbe.throwing('cdc', false))); }
}
