@IsTest private class GardenMixedWorkerTest52 {
    private class Leaf extends GardenVirtualWorker61 {
        private override String tag() { return 'leaf'; }
    }
    @IsTest static void declaringVersionControlsDispatch() {
        System.assertEquals('base', new Leaf().run());
    }
}
