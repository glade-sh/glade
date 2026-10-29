@IsTest private class FaultCtorTest {
    @IsTest static void implicitSuper() {
        PlainFaultException none = new PlainFaultException();
        PlainFaultException text = new PlainFaultException('owned text');
        PlainFaultException cause = new PlainFaultException(new DmlException('inner'));
        PlainFaultException both = new PlainFaultException('outer', new DmlException('inner'));
        System.assert(false, 'OBSERVED|implicitSuper|' + JSON.serialize(new Map<String, Object>{
            'none' => none.getMessage(),
            'noneDetail' => none.detail,
            'text' => text.getMessage(),
            'causeMessage' => cause.getMessage(),
            'causeInner' => cause.getCause().getMessage(),
            'both' => both.getMessage(),
            'bothInner' => both.getCause().getMessage()
        }));
    }
}
