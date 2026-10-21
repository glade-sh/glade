@IsTest private class GardenVirtualWorkerTest61 {
    private class Leaf extends GardenVirtualWorker61 {
        private override String tag() { return 'leaf'; }
    }
    private class PrivateLeaf extends GardenPrivateWorker61 {
        private String tag() { return 'leaf'; }
    }
    @IsTest static void nestedPrivateOverride() {
        Leaf leaf = new Leaf();
        GardenVirtualWorker61 baseView = leaf;
        System.assertEquals('base', leaf.run());
        System.assertEquals('base', baseView.run());
    }
    @IsTest static void nonVirtualControl() {
        System.assertEquals('base', new PrivateLeaf().run());
    }
}
