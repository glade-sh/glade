AcctHolder h = new AcctHolder();
List<String> out = new List<String>();
try { out.add('d01_acctChainId=' + h.acct.Id); } catch (Exception e) { out.add('d01_acctChainId=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d02_acctChainOwnerId=' + h.acct.OwnerId); } catch (Exception e) { out.add('d02_acctChainOwnerId=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d03_acctChainName=' + h.acct.Name); } catch (Exception e) { out.add('d03_acctChainName=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d04_acctChainCreatedDate=' + h.acct.CreatedDate); } catch (Exception e) { out.add('d04_acctChainCreatedDate=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d05_acctChainIsDeleted=' + h.acct.IsDeleted); } catch (Exception e) { out.add('d05_acctChainIsDeleted=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d06_acctChainWrite=' + (h.acct.Name = 'x')); } catch (Exception e) { out.add('d06_acctChainWrite=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d07_acctChainParentId=' + h.acct.ParentId); } catch (Exception e) { out.add('d07_acctChainParentId=EX ' + e.getTypeName() + '|' + e.getMessage()); }
try { out.add('d08_acctChainParentName=' + h.acct.Parent.Name); } catch (Exception e) { out.add('d08_acctChainParentName=EX ' + e.getTypeName() + '|' + e.getMessage()); }
System.debug('OBSERVED_D\n' + String.join(out, '\n'));
