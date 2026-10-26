@IsTest private class NumericRouteObservationTest {
    @IsTest static void observe() { System.assertEquals('NUMERIC_ROWS',JSON.serialize(NumericRouteObservation.observe())); }
    @IsTest static void grapheme() { System.assertEquals('GRAPHEME_ROWS',JSON.serialize(NumericRouteObservation.grapheme())); }
}
