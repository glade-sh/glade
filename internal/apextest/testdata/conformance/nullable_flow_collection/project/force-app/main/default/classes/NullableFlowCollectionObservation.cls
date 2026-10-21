@IsTest private class NullableFlowCollectionObservation {
    private static String observe(String sourceText) {
        Flow.Interview interview = Flow.Interview.createInterview('ParcelTextCollection', new Map<String,Object>{'sourceText'=>sourceText});
        interview.start();
        return JSON.serialize(interview.getVariableValue('result'));
    }
    @IsTest static void nullText() {
        String observed = observe(null);
        System.assert(false, 'OBSERVED|nullText|' + observed + '|END');
    }
    @IsTest static void text() {
        String observed = observe('parcel');
        System.assert(false, 'OBSERVED|text|' + observed + '|END');
    }
}
