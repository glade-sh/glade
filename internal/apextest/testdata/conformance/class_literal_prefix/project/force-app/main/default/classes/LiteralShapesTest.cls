@IsTest
private class LiteralShapesTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(LiteralShapes.observe())); }
}
