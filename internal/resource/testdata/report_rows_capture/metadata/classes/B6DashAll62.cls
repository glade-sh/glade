@IsTest(SeeAllData=true) private class B6DashAll62 {
 @IsTest static void capture() {
  List<Dashboard> ds = [SELECT Title, DeveloperName, FolderName, Type FROM Dashboard WHERE DeveloperName IN ('B6_Dashboard','B6_Other_Dashboard') ORDER BY DeveloperName];
  String result = 'count=' + ds.size();
  for (Dashboard d : ds) result += ';' + d.Title + ',' + d.DeveloperName + ',' + d.FolderName + ',' + d.Type;
  System.assert(false, result);
 }
}
