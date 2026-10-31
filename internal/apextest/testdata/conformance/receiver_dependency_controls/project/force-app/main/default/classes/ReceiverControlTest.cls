@IsTest private class ReceiverControlTest {
    @IsTest static void loaded() {
        System.assert(false, 'OBSERVED|loaded|' + JSON.serialize(new Map<String, Object>{
            'instance' => new ReceiverControl().instanceValue(),
            'static' => ReceiverControl.staticValue()
        }));
    }
    @IsTest static void nullBoundary() {
        ReceiverDependency receiver = null;
        String kind = 'none';
        try { receiver.value(); } catch (NullPointerException e) { kind = e.getTypeName(); }
        System.assert(false, 'OBSERVED|nullBoundary|' + JSON.serialize(new Map<String, Object>{'exception' => kind}));
    }
}
