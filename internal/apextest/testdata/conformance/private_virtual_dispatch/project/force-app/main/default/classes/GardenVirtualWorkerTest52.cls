@IsTest private class GardenVirtualWorkerTest52 {
    private class Leaf extends GardenVirtualWorker52 {
        private override String tag() { return 'leaf'; }
    }
    private class PrivateLeaf extends GardenPrivateWorker52 {
        private String tag() { return 'leaf'; }
    }
    @IsTest static void nestedPrivateOverride() {
        Leaf leaf = new Leaf();
        GardenVirtualWorker52 baseView = leaf;
        System.assertEquals('leaf', leaf.run());
        System.assertEquals('leaf', baseView.run());
    }
    @IsTest static void nonVirtualControl() {
        System.assertEquals('leaf', new PrivateLeaf().run());
    }
}
