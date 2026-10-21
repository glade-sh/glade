@IsTest private class GardenVirtualWorkerTest60 {
    private class Leaf extends GardenVirtualWorker60 {
        private override String tag() { return 'leaf'; }
    }
    private class PrivateLeaf extends GardenPrivateWorker60 {
        private String tag() { return 'leaf'; }
    }
    @IsTest static void nestedPrivateOverride() {
        Leaf leaf = new Leaf();
        GardenVirtualWorker60 baseView = leaf;
        System.assertEquals('leaf', leaf.run());
        System.assertEquals('leaf', baseView.run());
    }
    @IsTest static void nonVirtualControl() {
        System.assertEquals('leaf', new PrivateLeaf().run());
    }
}
