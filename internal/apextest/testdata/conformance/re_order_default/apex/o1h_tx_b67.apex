List<String> INS=new List<String>{'YB000','YB001','YB002','YB003','YB004','YB005','YB006','YB007','YB008','YB009','YB010','YB011','YB012','YB013','YB014','YB015','YB016','YB017','YB018','YB019','YB020','YB021','YB022','YB023','YB024','YB025','YB026','YB027','YB028','YB029','YB030','YB031','YB032','YB033','YB034','YB035','YB036','YB037','YB038','YB039','YB040','YB041','YB042','YB043','YB044','YB045','YB046','YB047','YB048','YB049','YB050','YB051','YB052','YB053','YB054','YB055','YB056','YB057','YB058','YB059','YB060','YB061','YB062','YB063','YB064','YB065','YB066','YB067','YB068','YB069','YB070','YB071','YB072','YB073','YB074','YB075','YB076','YB077','YB078','YB079','YB080','YB081','YB082','YB083','YB084','YB085','YB086','YB087','YB088','YB089','YB090','YB091','YB092','YB093','YB094','YB095','YB096','YB097','YB098','YB099','YB100','YB101','YB102','YB103','YB104','YB105','YB106','YB107','YB108','YB109','YB110','YB111','YB112','YB113','YB114','YB115','YB116','YB117','YB118','YB119','YB120','YB121','YB122','YB123','YB124','YB125','YB126','YB127','YB128','YB129','YB130','YB131','YB132','YB133','YB134','YB135','YB136'};
List<Ord__c> rows=new List<Ord__c>(); for(String n:INS){rows.add(new Ord__c(Name=n));}
insert rows;
List<Ord__c> q=[SELECT Id,Name FROM Ord__c];
List<String> qn=new List<String>(); List<String> qi=new List<String>();
for(Ord__c r:q){qn.add(r.Name);qi.add(String.valueOf(r.Id));}
System.debug('O1|TXB|IN|'+String.join(INS,','));
System.debug('O1|TXB|Q|'+String.join(qn,','));
System.debug('O1|TXB|IDS|'+String.join(qi,','));
