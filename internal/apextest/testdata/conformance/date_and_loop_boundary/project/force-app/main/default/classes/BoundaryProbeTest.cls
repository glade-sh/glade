@IsTest private class BoundaryProbeTest {
    @IsTest static void dateLiteral() { emit('dateLiteral',BoundaryProbe.dateLiteral()); }
    @IsTest static void dateVariable() { emit('dateVariable',BoundaryProbe.dateVariable()); }
    @IsTest static void datetimeControl() { emit('datetimeControl',BoundaryProbe.datetimeControl()); }
    @IsTest static void loopBelow() { emit('loopBelow',BoundaryProbe.loopBelow()); }
    @IsTest static void loopAbove() { emit('loopAbove',BoundaryProbe.loopAbove()); }
    @IsTest static void timeAdvance() { emit('timeAdvance',BoundaryProbe.timeAdvance()); }
    private static void emit(String method,Map<String,Object> observed) {
        System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(observed));
    }
}
