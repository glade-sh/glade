@IsTest private class GardenMixedWorkerTest67 {
    private class Leaf extends GardenVirtualWorker60 {
        protected override String tag() { return 'leaf'; }
    }
    @IsTest static void declaringVersionControlsDispatch() {
        System.assertEquals('leaf', new Leaf().run());
    }
}
