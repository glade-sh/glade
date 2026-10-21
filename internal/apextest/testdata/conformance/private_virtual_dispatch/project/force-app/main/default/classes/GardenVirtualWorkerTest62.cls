@IsTest private class GardenVirtualWorkerTest62 {
    private class Leaf extends GardenVirtualWorker62 {
        private override String tag() { return 'leaf'; }
    }
    private class PrivateLeaf extends GardenPrivateWorker62 {
        private String tag() { return 'leaf'; }
    }
    @IsTest static void nestedPrivateOverride() {
        Leaf leaf = new Leaf();
        GardenVirtualWorker62 baseView = leaf;
        System.assertEquals('base', leaf.run());
        System.assertEquals('base', baseView.run());
    }
    @IsTest static void nonVirtualControl() {
        System.assertEquals('base', new PrivateLeaf().run());
    }
}
