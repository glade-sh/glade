Map<Id,Folder> folders = new Map<Id,Folder>([SELECT Id,Name,DeveloperName,Type FROM Folder WHERE DeveloperName IN ('B6_Dashboards','B6_Another')]);
for (Dashboard d : [SELECT DeveloperName,FolderId,FolderName FROM Dashboard WHERE DeveloperName IN ('B6_Dashboard','B6_Other_Dashboard') ORDER BY DeveloperName]) {
 Folder f=folders.get(d.FolderId);
 System.debug('B6LINK|'+d.DeveloperName+'|'+(f==null?'missing':f.DeveloperName)+'|'+(f==null?'missing':f.Name)+'|'+d.FolderName);
}
