@IsTest private class CountedFaultTest {
    @IsTest static void explicitImplicitSuper() {
        CountedFaultException value = new CountedFaultException(4);
        System.assert(false, 'OBSERVED|explicitImplicitSuper|' + JSON.serialize(new Map<String, Object>{
            'message' => value.getMessage(),
            'detail' => value.detail
        }));
    }
}
