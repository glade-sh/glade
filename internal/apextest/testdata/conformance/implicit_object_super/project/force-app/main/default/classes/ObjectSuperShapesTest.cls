@IsTest
private class ObjectSuperShapesTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ObjectSuperShapes.observe())); }
}
