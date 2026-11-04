@IsTest public class NullNumActionProbeTest {
    public class ObservationException extends Exception {}
    @IsTest static void action() { throw new ObservationException('PROBE|' + NullNumActionProbe.observe()); }
}
