@IsTest private class GardenVirtualWorkerTest67 {
    private class Leaf extends GardenVirtualWorker67 {
        protected override String tag() { return 'leaf'; }
    }
    private class PrivateLeaf extends GardenPrivateWorker67 {
        private String tag() { return 'leaf'; }
    }
    @IsTest static void nestedPrivateOverride() {
        Leaf leaf = new Leaf();
        GardenVirtualWorker67 baseView = leaf;
        System.assertEquals('base', leaf.run());
        System.assertEquals('base', baseView.run());
    }
    @IsTest static void nonVirtualControl() {
        System.assertEquals('base', new PrivateLeaf().run());
    }
}
