List<Report> rs = [SELECT Id, Name, DeveloperName, FolderName, Format FROM Report WHERE DeveloperName = 'B6_Accounts'];
System.debug('B6|count|' + rs.size());
if (!rs.isEmpty()) {
 Report r = rs[0];
 System.debug('B6|report|' + r.Name + '|' + r.DeveloperName + '|' + r.FolderName + '|' + r.Format);
 try { Reports.ReportMetadata m = Reports.ReportManager.describeReport(r.Id).getReportMetadata(); System.debug('B6|describe|' + m.getName() + '|' + m.getReportFormat() + '|' + m.getDetailColumns()); } catch (Exception e) { System.debug('B6|describe-error|' + e.getTypeName() + '|' + e.getMessage()); }
 try { Reports.ReportResults x = Reports.ReportManager.runReport(r.Id, true); System.debug('B6|run|' + x.getReportMetadata().getName() + '|' + x.getFactMap().size()); } catch (Exception e) { System.debug('B6|run-error|' + e.getTypeName() + '|' + e.getMessage()); }
}
try { Reports.ReportManager.describeReport((Id)null); } catch (Exception e) { System.debug('B6|null|' + e.getTypeName() + '|' + e.getMessage()); }
