@IsTest private class FailureOverrideTest {
    private class DetailFailureException extends BaseFailureException {
        public override String getMessage() { return 'owned detail'; }
    }
    @IsTest static void indirectOverride() {
        Exception value = new DetailFailureException('base message');
        System.assert(false, 'OBSERVED|indirectOverride|' + JSON.serialize(new Map<String,Object>{'message'=>value.getMessage(),'stack'=>value.getStackTraceString()}));
    }
}
