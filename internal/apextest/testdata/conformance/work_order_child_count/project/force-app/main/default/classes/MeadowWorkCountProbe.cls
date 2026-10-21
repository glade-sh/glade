public class MeadowWorkCountProbe {
 public static String sample() {
  Savepoint beginning = Database.setSavepoint();
  WorkOrder first = new WorkOrder(Subject='Meadow service');
  WorkOrder second = new WorkOrder(Subject='Brook service');
  insert new List<WorkOrder>{first, second};
  Map<String,Object> rows = new Map<String,Object>();
  rows.put('empty', countFor(first.Id));
  List<WorkOrderLineItem> items = new List<WorkOrderLineItem>{
   new WorkOrderLineItem(WorkOrderId=first.Id, Description='Meadow first'),
   new WorkOrderLineItem(WorkOrderId=first.Id, Description='Meadow second')
  };
  insert items;
  rows.put('inserted', countFor(first.Id));
  rows.put('otherParent', countFor(second.Id));
  Savepoint inserted = Database.setSavepoint();
  delete items[0];
  rows.put('deleted', countFor(first.Id));
  undelete items[0];
  rows.put('undeleted', countFor(first.Id));
  Database.rollback(inserted);
  rows.put('rollbackOld', countFor(first.Id));
  rows.put('rollbackNew', countFor(second.Id));
  Database.rollback(beginning);
  return JSON.serialize(rows);
 }
 private static Integer countFor(Id parentId) {
  return [SELECT LineItemCount FROM WorkOrder WHERE Id=:parentId].LineItemCount;
 }
}
