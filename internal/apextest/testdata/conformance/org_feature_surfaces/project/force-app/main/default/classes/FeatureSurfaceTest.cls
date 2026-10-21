@IsTest private class FeatureSurfaceTest {
    @IsTest static void capture() {System.assert(false,'OBSERVED|capture|'+JSON.serialize(FeatureSurfaceProbe.observe()));}
}
