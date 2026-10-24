@IsTest private class StackDepthProbeTest {
    @IsTest static void capped_plain_1() {
        Test.startTest();
        StackDepthProbe.submit('capped_plain_1', 'queue', 1, 2, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('capped_plain_1')));
    }
    @IsTest static void capped_plain_2() {
        Test.startTest();
        StackDepthProbe.submit('capped_plain_2', 'queue', 2, 2, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('capped_plain_2')));
    }
    @IsTest static void capped_plain_3() {
        Test.startTest();
        StackDepthProbe.submit('capped_plain_3', 'queue', 3, 2, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('capped_plain_3')));
    }
    @IsTest static void capped_delay_1() {
        Test.startTest();
        StackDepthProbe.submit('capped_delay_1', 'queue', 1, 2, 'delay', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('capped_delay_1')));
    }
    @IsTest static void capped_delay_2() {
        Test.startTest();
        StackDepthProbe.submit('capped_delay_2', 'queue', 2, 2, 'delay', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('capped_delay_2')));
    }
    @IsTest static void capped_delay_3() {
        Test.startTest();
        StackDepthProbe.submit('capped_delay_3', 'queue', 3, 2, 'delay', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('capped_delay_3')));
    }
    @IsTest static void capped_options_1() {
        Test.startTest();
        StackDepthProbe.submit('capped_options_1', 'queue', 1, 2, 'options', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('capped_options_1')));
    }
    @IsTest static void capped_options_2() {
        Test.startTest();
        StackDepthProbe.submit('capped_options_2', 'queue', 2, 2, 'options', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('capped_options_2')));
    }
    @IsTest static void capped_options_3() {
        Test.startTest();
        StackDepthProbe.submit('capped_options_3', 'queue', 3, 2, 'options', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('capped_options_3')));
    }
    @IsTest static void inherited_one() {
        Test.startTest();
        StackDepthProbe.submit('inherited_one', 'queue', 2, 1, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('inherited_one')));
    }
    @IsTest static void uncapped_one() {
        Test.startTest();
        StackDepthProbe.submit('uncapped_one', 'queue', 1, null, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('uncapped_one')));
    }
    @IsTest static void uncapped_child() {
        Test.startTest();
        StackDepthProbe.submit('uncapped_child', 'queue', 2, null, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('uncapped_child')));
    }
    @IsTest static void uncapped_grandchild() {
        Test.startTest();
        StackDepthProbe.submit('uncapped_grandchild', 'queue', 3, null, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('uncapped_grandchild')));
    }
    @IsTest static void zero_cap() {
        Test.startTest();
        StackDepthProbe.submit('zero_cap', 'queue', 1, 0, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('zero_cap')));
    }
    @IsTest static void future_1() {
        Test.startTest();
        StackDepthProbe.submit('future_1', 'future', 1, null, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('future_1')));
    }
    @IsTest static void future_2() {
        Test.startTest();
        StackDepthProbe.submit('future_2', 'future', 2, null, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('future_2')));
    }
    @IsTest static void future_3() {
        Test.startTest();
        StackDepthProbe.submit('future_3', 'future', 3, null, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('future_3')));
    }
    @IsTest static void batch_1() {
        Test.startTest();
        StackDepthProbe.submit('batch_1', 'batch', 1, null, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('batch_1')));
    }
    @IsTest static void batch_2() {
        Test.startTest();
        StackDepthProbe.submit('batch_2', 'batch', 2, null, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('batch_2')));
    }
    @IsTest static void batch_3() {
        Test.startTest();
        StackDepthProbe.submit('batch_3', 'batch', 3, null, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('batch_3')));
    }
    @IsTest static void mixed_four() {
        Test.startTest();
        StackDepthProbe.submit('mixed_four', 'mixed', 4, 4, 'plain', true);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('mixed_four')));
    }
    @IsTest static void overflow_flush() {
        Test.startTest();
        StackDepthProbe.submit('overflow_flush', 'queue', 2, 1, 'plain', false);
        Test.stopTest();
        System.assert(false, 'STACK|'+JSON.serialize(StackDepthProbe.readback('overflow_flush')));
    }
}
