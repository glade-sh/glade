@IsTest private class CaptureDmlSequenceTest {
    @IsTest static void observe() { List<List<Integer>> rows=CaptureDmlSequence.observe(); System.debug('OBS|'+JSON.serialize(rows)); System.assert(false, 'OBS|'+JSON.serialize(rows)); }
    @IsTest static void overflow() { CaptureDmlSequence.overflow(); }
}
