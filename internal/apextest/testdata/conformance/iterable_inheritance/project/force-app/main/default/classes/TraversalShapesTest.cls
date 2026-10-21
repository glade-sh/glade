@IsTest
private class TraversalShapesTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(TraversalShapes.observe())); }
}
