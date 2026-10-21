@IsTest private class AliasedRemoval67 {
 private List<Integer> items;
 private AliasedRemoval67(Integer count) {
  items = new List<Integer>{1};
  if (count == 2) items.add(2);
 }
 private List<Integer> getItems() { return items; }
 private void drop(Integer index) { items.remove(index); }
 @IsTest static void directOne() { direct(1); }
 @IsTest static void directTwo() { direct(2); }
 @IsTest static void aliasOne() { aliased(1); }
 @IsTest static void aliasTwo() { aliased(2); }
 private static void direct(Integer count) {
  List<Integer> values = new List<Integer>{1};
  if (count == 2) values.add(2);
  for (Integer item : values) {
   for (Integer target : new List<Integer>{1}) {
    if (item == target) values.remove(values.indexOf(item));
   }
  }
  System.assert(false, 'ALIASED_REMOVAL|completed|' + values.size());
 }
 private static void aliased(Integer count) {
  AliasedRemoval67 holder = new AliasedRemoval67(count);
  List<Integer> values = holder.getItems();
  for (Integer item : values) {
   for (Integer target : new List<Integer>{1}) {
    if (item == target) holder.drop(values.indexOf(item));
   }
  }
  System.assert(false, 'ALIASED_REMOVAL|completed|' + values.size());
 }
}
