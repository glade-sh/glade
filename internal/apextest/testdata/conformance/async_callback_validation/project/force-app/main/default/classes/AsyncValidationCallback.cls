public class AsyncValidationCallback extends DataSource.AsyncSaveCallback {
 public static Integer calls=0;
 public override void processSave(Database.SaveResult result) {calls++;}
}
