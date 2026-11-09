@IsTest private class B6FoldersIsolated62 {
 @IsTest static void capture() {
  Integer n = [SELECT COUNT() FROM Folder WHERE DeveloperName IN ('B6_Reports','B6_Dashboards')];
  System.assert(false, 'count=' + n);
 }
}
