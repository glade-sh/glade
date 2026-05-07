@isTest
private class MetadataDeployTest {
  @isTest static void deploysCustomObjectAndFieldsIntoLocalSchema() {
    Metadata.DeployContainer container = new Metadata.DeployContainer();

    Metadata.CustomObject objectDef = new Metadata.CustomObject();
    objectDef.fullName = 'Invoice__c';
    objectDef.label = 'Invoice';
    objectDef.pluralLabel = 'Invoices';
    container.addMetadata(objectDef);

    Metadata.CustomField amount = new Metadata.CustomField();
    amount.fullName = 'Invoice__c.Amount__c';
    amount.label = 'Amount';
    amount.type = 'Number';
    amount.required = true;
    container.addMetadata(amount);

    Metadata.CustomField paid = new Metadata.CustomField();
    paid.fullName = 'Invoice__c.Paid__c';
    paid.label = 'Paid';
    paid.type = 'Checkbox';
    container.addMetadata(paid);

    String deploymentId = Metadata.Operations.enqueueDeployment(container, null);
    Metadata.DeployResult result = Metadata.Operations.checkDeployStatus(deploymentId, true);
    System.assert(result.success);
    System.assertEquals(3, result.details.componentSuccesses.size());
    System.assertEquals('CustomObject', result.details.componentSuccesses[0].componentType);
    System.assertEquals('CustomField', result.details.componentSuccesses[1].componentType);

    Map<String, Object> describes = Schema.getGlobalDescribe();
    System.assert(describes.containsKey('Invoice__c'));

    List<SObject> rows = Database.query('SELECT Id, Amount__c, Paid__c FROM Invoice__c');
    System.assertEquals(0, rows.size());
  }
}
