public class PlatformCompleteBatch extends UserProvisioning.PluginBatchable {
 public PlatformCompleteBatch() { super(new List<SObject>()); }

 public override String getEventPrefix() { return 'owned'; }
 public override Map<String,Object> flowInputPreprocessing(Map<String,Object> values) { values.put('hook',true); return values; }
 public override void flowPostProcessing(UserProvisioning.ProvisioningProcessHandlerOutput output,SObject row) {}
 public override void postBatchProcessing() {}
 public void finish(Database.BatchableContext context) { insert new Account(Name='Platform batch finish'); }
}
