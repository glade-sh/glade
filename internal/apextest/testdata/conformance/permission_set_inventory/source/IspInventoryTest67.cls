@isTest
private class IspInventoryTest67 {
    @isTest static void capture_matrix() {
        IspInventory67.beginCapture('test');
        User administrator=[SELECT Id FROM User WHERE Id=:UserInfo.getUserId() WITH SYSTEM_MODE];
        // Genuine test-only admin runAs; no users, permissions or fixture objects are created.
        System.runAs(administrator) {IspInventory67.capture();}
        String carrier=IspInventory67.finishCapture();
        System.assert(false,carrier);
    }
}
