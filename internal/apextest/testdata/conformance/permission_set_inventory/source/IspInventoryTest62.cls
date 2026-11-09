@isTest
private class IspInventoryTest62 {
    @isTest static void capture_matrix() {
        IspInventory62.beginCapture('test');
        User administrator=[SELECT Id FROM User WHERE Id=:UserInfo.getUserId() WITH SYSTEM_MODE];
        // Genuine test-only admin runAs; no users, permissions or fixture objects are created.
        System.runAs(administrator) {IspInventory62.capture();}
        String carrier=IspInventory62.finishCapture();
        System.assert(false,carrier);
    }
}
