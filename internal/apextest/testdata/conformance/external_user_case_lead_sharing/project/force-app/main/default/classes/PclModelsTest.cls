@isTest private class PclModelsTest {
    @isTest static void run() {
        String m = '';
        for (EntityDefinition e : [SELECT QualifiedApiName, InternalSharingModel, ExternalSharingModel FROM EntityDefinition WHERE QualifiedApiName IN ('Case','Lead') ORDER BY QualifiedApiName]) { m += e.QualifiedApiName + ':' + e.InternalSharingModel + '/' + e.ExternalSharingModel + ';'; }
        System.assert(false, m);
    }
}
