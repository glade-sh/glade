@IsTest
private class DthTz {
    @IsTest static void T_tz_gmt() {
        User u = new User(
            Alias = 'dthgmt', Email = 'dthuser1@example.invalid', EmailEncodingKey = 'UTF-8',
            LastName = 'Dth gmt', LanguageLocaleKey = 'en_US',
            LocaleSidKey = 'en_US', ProfileId = DthHashSupport.profileId(),
            TimeZoneSidKey = 'GMT', UserName = 'dthuser2@example.invalid.glade');
        insert u;
        DthHashSupport.grantProbe(u.Id);
        System.runAs(u) {
            String p = 'tz=GMT';
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(0, 0, 0, 0));
                p += '|i01=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 0L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i01=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(0, 0, 0, 1));
                p += '|i02=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i02=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(0, 0, 0, 999));
                p += '|i03=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i03=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(0, 0, 1, 0));
                p += '|i04=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i04=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 12, 31), Time.newInstance(23, 59, 59, 999));
                p += '|i05=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -1L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i05=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1900, 1, 1), Time.newInstance(0, 0, 0, 0));
                p += '|i06=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -2208988800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i06=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2038, 1, 19), Time.newInstance(3, 14, 8, 0));
                p += '|i07=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483648000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i07=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(4000, 12, 31), Time.newInstance(0, 0, 0, 0));
                p += '|i08=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 64092124800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i08=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(4000, 12, 31), Time.newInstance(23, 59, 59, 999));
                p += '|i09=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 64092211199999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i09=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1905, 6, 15), Time.newInstance(8, 7, 6, 543));
                p += '|i10=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -2036937173457L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i10=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1982, 1, 20), Time.newInstance(4, 2, 12, 415));
                p += '|i11=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 380347332415L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i11=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1993, 10, 4), Time.newInstance(5, 25, 48, 663));
                p += '|i12=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 749712348663L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i12=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2009, 12, 23), Time.newInstance(20, 12, 34, 14));
                p += '|i13=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1261599154014L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i13=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2018, 5, 3), Time.newInstance(4, 42, 44, 705));
                p += '|i14=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1525322564705L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i14=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2047, 6, 29), Time.newInstance(19, 59, 40, 644));
                p += '|i15=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2445451180644L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i15=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2085, 1, 10), Time.newInstance(15, 18, 19, 962));
                p += '|i16=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 3629978299962L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i16=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2178, 7, 26), Time.newInstance(0, 4, 25, 534));
                p += '|i17=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 6581693065534L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i17=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2356, 6, 10), Time.newInstance(19, 53, 22, 366));
                p += '|i18=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 12194913202366L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i18=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2543, 5, 23), Time.newInstance(15, 15, 18, 853));
                p += '|i19=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 18094461318853L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i19=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(3093, 11, 11), Time.newInstance(6, 40, 40, 38));
                p += '|i20=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 35465668840038L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i20=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 25), Time.newInstance(20, 31, 23, 647));
                p += '|i21=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483647L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i21=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 25), Time.newInstance(20, 31, 23, 648));
                p += '|i22=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483648L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i22=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 2, 19), Time.newInstance(17, 2, 47, 295));
                p += '|i23=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 4294967295L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i23=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 2, 19), Time.newInstance(17, 2, 47, 296));
                p += '|i24=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 4294967296L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i24=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 11, 12), Time.newInstance(6, 57, 12, 704));
                p += '|i25=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -4294967296L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i25=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 11, 12), Time.newInstance(6, 57, 12, 703));
                p += '|i26=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -4294967297L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i26=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 14), Time.newInstance(23, 59, 59, 999));
                p += '|i27=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292800001L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i27=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 15), Time.newInstance(0, 0, 0, 0));
                p += '|i28=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i28=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 15), Time.newInstance(0, 0, 0, 1));
                p += '|i29=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292799999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i29=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Account acc = new Account(Name = 'dth tz row');
                insert acc;
                Datetime cd = [SELECT CreatedDate FROM Account WHERE Id = :acc.Id].CreatedDate;
                p += '|SOQL=' + cd.hashCode() + ';' + cd.getTime() + ';' + System.hashCode(cd);
                Dth_Probe__c r = new Dth_Probe__c(Name = 'dth tz row', Dt__c = cd);
                p += '|FM=' + r.Dt__c.hashCode() + ';' + r.Dt__c.getTime() + ';' + System.hashCode(r.Dt__c);
                insert r;
                Dth_Probe__c rq = [SELECT Dt__c FROM Dth_Probe__c WHERE Id = :r.Id];
                p += '|FR=' + rq.Dt__c.hashCode() + ';' + rq.Dt__c.getTime() + ';' + System.hashCode(rq.Dt__c);
            } catch (Exception e) {
                p += '|DML=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            System.assert(false, p);
        }
    }
    @IsTest static void T_tz_la() {
        User u = new User(
            Alias = 'dthla', Email = 'dthuser3@example.invalid', EmailEncodingKey = 'UTF-8',
            LastName = 'Dth la', LanguageLocaleKey = 'en_US',
            LocaleSidKey = 'en_US', ProfileId = DthHashSupport.profileId(),
            TimeZoneSidKey = 'America/Los_Angeles', UserName = 'dthuser4@example.invalid.glade');
        insert u;
        DthHashSupport.grantProbe(u.Id);
        System.runAs(u) {
            String p = 'tz=America/Los_Angeles';
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 12, 31), Time.newInstance(16, 0, 0, 0));
                p += '|i01=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 0L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i01=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 12, 31), Time.newInstance(16, 0, 0, 1));
                p += '|i02=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i02=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 12, 31), Time.newInstance(16, 0, 0, 999));
                p += '|i03=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i03=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 12, 31), Time.newInstance(16, 0, 1, 0));
                p += '|i04=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i04=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 12, 31), Time.newInstance(15, 59, 59, 999));
                p += '|i05=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -1L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i05=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1899, 12, 31), Time.newInstance(16, 0, 0, 0));
                p += '|i06=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -2208988800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i06=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2038, 1, 18), Time.newInstance(19, 14, 8, 0));
                p += '|i07=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483648000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i07=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(4000, 12, 30), Time.newInstance(16, 0, 0, 0));
                p += '|i08=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 64092124800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i08=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(4000, 12, 31), Time.newInstance(15, 59, 59, 999));
                p += '|i09=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 64092211199999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i09=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1905, 6, 15), Time.newInstance(0, 7, 6, 543));
                p += '|i10=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -2036937173457L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i10=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1982, 1, 19), Time.newInstance(20, 2, 12, 415));
                p += '|i11=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 380347332415L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i11=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1993, 10, 3), Time.newInstance(22, 25, 48, 663));
                p += '|i12=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 749712348663L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i12=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2009, 12, 23), Time.newInstance(12, 12, 34, 14));
                p += '|i13=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1261599154014L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i13=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2018, 5, 2), Time.newInstance(21, 42, 44, 705));
                p += '|i14=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1525322564705L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i14=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2047, 6, 29), Time.newInstance(12, 59, 40, 644));
                p += '|i15=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2445451180644L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i15=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2085, 1, 10), Time.newInstance(7, 18, 19, 962));
                p += '|i16=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 3629978299962L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i16=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2178, 7, 25), Time.newInstance(17, 4, 25, 534));
                p += '|i17=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 6581693065534L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i17=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2356, 6, 10), Time.newInstance(12, 53, 22, 366));
                p += '|i18=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 12194913202366L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i18=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2543, 5, 23), Time.newInstance(8, 15, 18, 853));
                p += '|i19=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 18094461318853L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i19=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(3093, 11, 10), Time.newInstance(22, 40, 40, 38));
                p += '|i20=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 35465668840038L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i20=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 25), Time.newInstance(12, 31, 23, 647));
                p += '|i21=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483647L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i21=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 25), Time.newInstance(12, 31, 23, 648));
                p += '|i22=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483648L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i22=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 2, 19), Time.newInstance(9, 2, 47, 295));
                p += '|i23=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 4294967295L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i23=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 2, 19), Time.newInstance(9, 2, 47, 296));
                p += '|i24=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 4294967296L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i24=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 11, 11), Time.newInstance(22, 57, 12, 704));
                p += '|i25=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -4294967296L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i25=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 11, 11), Time.newInstance(22, 57, 12, 703));
                p += '|i26=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -4294967297L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i26=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 14), Time.newInstance(16, 7, 1, 999));
                p += '|i27=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292800001L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i27=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 14), Time.newInstance(16, 7, 2, 0));
                p += '|i28=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i28=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 14), Time.newInstance(16, 7, 2, 1));
                p += '|i29=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292799999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i29=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Account acc = new Account(Name = 'dth tz row');
                insert acc;
                Datetime cd = [SELECT CreatedDate FROM Account WHERE Id = :acc.Id].CreatedDate;
                p += '|SOQL=' + cd.hashCode() + ';' + cd.getTime() + ';' + System.hashCode(cd);
                Dth_Probe__c r = new Dth_Probe__c(Name = 'dth tz row', Dt__c = cd);
                p += '|FM=' + r.Dt__c.hashCode() + ';' + r.Dt__c.getTime() + ';' + System.hashCode(r.Dt__c);
                insert r;
                Dth_Probe__c rq = [SELECT Dt__c FROM Dth_Probe__c WHERE Id = :r.Id];
                p += '|FR=' + rq.Dt__c.hashCode() + ';' + rq.Dt__c.getTime() + ';' + System.hashCode(rq.Dt__c);
            } catch (Exception e) {
                p += '|DML=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            System.assert(false, p);
        }
    }
    @IsTest static void T_tz_in() {
        User u = new User(
            Alias = 'dthin', Email = 'dthuser5@example.invalid', EmailEncodingKey = 'UTF-8',
            LastName = 'Dth in', LanguageLocaleKey = 'en_US',
            LocaleSidKey = 'en_US', ProfileId = DthHashSupport.profileId(),
            TimeZoneSidKey = 'Asia/Kolkata', UserName = 'dthuser6@example.invalid.glade');
        insert u;
        DthHashSupport.grantProbe(u.Id);
        System.runAs(u) {
            String p = 'tz=Asia/Kolkata';
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(5, 30, 0, 0));
                p += '|i01=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 0L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i01=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(5, 30, 0, 1));
                p += '|i02=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i02=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(5, 30, 0, 999));
                p += '|i03=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i03=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(5, 30, 1, 0));
                p += '|i04=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i04=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(5, 29, 59, 999));
                p += '|i05=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -1L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i05=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1900, 1, 1), Time.newInstance(5, 21, 10, 0));
                p += '|i06=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -2208988800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i06=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2038, 1, 19), Time.newInstance(8, 44, 8, 0));
                p += '|i07=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483648000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i07=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(4000, 12, 31), Time.newInstance(5, 30, 0, 0));
                p += '|i08=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 64092124800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i08=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(4001, 1, 1), Time.newInstance(5, 29, 59, 999));
                p += '|i09=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 64092211199999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i09=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1905, 6, 15), Time.newInstance(13, 28, 16, 543));
                p += '|i10=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -2036937173457L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i10=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1982, 1, 20), Time.newInstance(9, 32, 12, 415));
                p += '|i11=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 380347332415L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i11=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1993, 10, 4), Time.newInstance(10, 55, 48, 663));
                p += '|i12=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 749712348663L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i12=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2009, 12, 24), Time.newInstance(1, 42, 34, 14));
                p += '|i13=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1261599154014L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i13=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2018, 5, 3), Time.newInstance(10, 12, 44, 705));
                p += '|i14=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1525322564705L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i14=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2047, 6, 30), Time.newInstance(1, 29, 40, 644));
                p += '|i15=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2445451180644L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i15=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2085, 1, 10), Time.newInstance(20, 48, 19, 962));
                p += '|i16=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 3629978299962L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i16=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2178, 7, 26), Time.newInstance(5, 34, 25, 534));
                p += '|i17=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 6581693065534L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i17=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2356, 6, 11), Time.newInstance(1, 23, 22, 366));
                p += '|i18=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 12194913202366L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i18=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2543, 5, 23), Time.newInstance(20, 45, 18, 853));
                p += '|i19=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 18094461318853L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i19=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(3093, 11, 11), Time.newInstance(12, 10, 40, 38));
                p += '|i20=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 35465668840038L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i20=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 26), Time.newInstance(2, 1, 23, 647));
                p += '|i21=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483647L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i21=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 26), Time.newInstance(2, 1, 23, 648));
                p += '|i22=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483648L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i22=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 2, 19), Time.newInstance(22, 32, 47, 295));
                p += '|i23=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 4294967295L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i23=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 2, 19), Time.newInstance(22, 32, 47, 296));
                p += '|i24=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 4294967296L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i24=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 11, 12), Time.newInstance(12, 27, 12, 704));
                p += '|i25=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -4294967296L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i25=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 11, 12), Time.newInstance(12, 27, 12, 703));
                p += '|i26=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -4294967297L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i26=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 15), Time.newInstance(5, 53, 27, 999));
                p += '|i27=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292800001L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i27=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 15), Time.newInstance(5, 53, 28, 0));
                p += '|i28=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i28=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 15), Time.newInstance(5, 53, 28, 1));
                p += '|i29=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292799999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i29=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Account acc = new Account(Name = 'dth tz row');
                insert acc;
                Datetime cd = [SELECT CreatedDate FROM Account WHERE Id = :acc.Id].CreatedDate;
                p += '|SOQL=' + cd.hashCode() + ';' + cd.getTime() + ';' + System.hashCode(cd);
                Dth_Probe__c r = new Dth_Probe__c(Name = 'dth tz row', Dt__c = cd);
                p += '|FM=' + r.Dt__c.hashCode() + ';' + r.Dt__c.getTime() + ';' + System.hashCode(r.Dt__c);
                insert r;
                Dth_Probe__c rq = [SELECT Dt__c FROM Dth_Probe__c WHERE Id = :r.Id];
                p += '|FR=' + rq.Dt__c.hashCode() + ';' + rq.Dt__c.getTime() + ';' + System.hashCode(rq.Dt__c);
            } catch (Exception e) {
                p += '|DML=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            System.assert(false, p);
        }
    }
    @IsTest static void T_tz_ad() {
        User u = new User(
            Alias = 'dthad', Email = 'dthuser7@example.invalid', EmailEncodingKey = 'UTF-8',
            LastName = 'Dth ad', LanguageLocaleKey = 'en_US',
            LocaleSidKey = 'en_US', ProfileId = DthHashSupport.profileId(),
            TimeZoneSidKey = 'Australia/Adelaide', UserName = 'dthuser8@example.invalid.glade');
        insert u;
        DthHashSupport.grantProbe(u.Id);
        System.runAs(u) {
            String p = 'tz=Australia/Adelaide';
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(9, 30, 0, 0));
                p += '|i01=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 0L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i01=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(9, 30, 0, 1));
                p += '|i02=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i02=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(9, 30, 0, 999));
                p += '|i03=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i03=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(9, 30, 1, 0));
                p += '|i04=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i04=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 1), Time.newInstance(9, 29, 59, 999));
                p += '|i05=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -1L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i05=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1900, 1, 1), Time.newInstance(9, 30, 0, 0));
                p += '|i06=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -2208988800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i06=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2038, 1, 19), Time.newInstance(13, 44, 8, 0));
                p += '|i07=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483648000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i07=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(4000, 12, 31), Time.newInstance(10, 30, 0, 0));
                p += '|i08=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 64092124800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i08=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(4001, 1, 1), Time.newInstance(10, 29, 59, 999));
                p += '|i09=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 64092211199999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i09=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1905, 6, 15), Time.newInstance(17, 37, 6, 543));
                p += '|i10=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -2036937173457L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i10=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1982, 1, 20), Time.newInstance(14, 32, 12, 415));
                p += '|i11=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 380347332415L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i11=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1993, 10, 4), Time.newInstance(14, 55, 48, 663));
                p += '|i12=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 749712348663L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i12=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2009, 12, 24), Time.newInstance(6, 42, 34, 14));
                p += '|i13=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1261599154014L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i13=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2018, 5, 3), Time.newInstance(14, 12, 44, 705));
                p += '|i14=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 1525322564705L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i14=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2047, 6, 30), Time.newInstance(5, 29, 40, 644));
                p += '|i15=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2445451180644L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i15=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2085, 1, 11), Time.newInstance(1, 48, 19, 962));
                p += '|i16=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 3629978299962L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i16=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2178, 7, 26), Time.newInstance(9, 34, 25, 534));
                p += '|i17=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 6581693065534L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i17=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2356, 6, 11), Time.newInstance(5, 23, 22, 366));
                p += '|i18=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 12194913202366L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i18=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(2543, 5, 24), Time.newInstance(0, 45, 18, 853));
                p += '|i19=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 18094461318853L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i19=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(3093, 11, 11), Time.newInstance(17, 10, 40, 38));
                p += '|i20=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 35465668840038L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i20=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 26), Time.newInstance(6, 1, 23, 647));
                p += '|i21=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483647L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i21=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 1, 26), Time.newInstance(6, 1, 23, 648));
                p += '|i22=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 2147483648L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i22=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 2, 20), Time.newInstance(2, 32, 47, 295));
                p += '|i23=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 4294967295L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i23=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1970, 2, 20), Time.newInstance(2, 32, 47, 296));
                p += '|i24=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == 4294967296L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i24=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 11, 12), Time.newInstance(16, 27, 12, 704));
                p += '|i25=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -4294967296L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i25=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1969, 11, 12), Time.newInstance(16, 27, 12, 703));
                p += '|i26=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -4294967297L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i26=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 15), Time.newInstance(9, 14, 19, 999));
                p += '|i27=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292800001L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i27=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 15), Time.newInstance(9, 14, 20, 0));
                p += '|i28=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292800000L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i28=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Datetime x = Datetime.newInstance(
                    Date.newInstance(1582, 10, 15), Time.newInstance(9, 14, 20, 1));
                p += '|i29=' + x.hashCode() + ';' + x.getTime() + ';' + (x.getTime() == -12219292799999L) + ';' + System.hashCode(x);
            } catch (Exception e) {
                p += '|i29=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            try {
                Account acc = new Account(Name = 'dth tz row');
                insert acc;
                Datetime cd = [SELECT CreatedDate FROM Account WHERE Id = :acc.Id].CreatedDate;
                p += '|SOQL=' + cd.hashCode() + ';' + cd.getTime() + ';' + System.hashCode(cd);
                Dth_Probe__c r = new Dth_Probe__c(Name = 'dth tz row', Dt__c = cd);
                p += '|FM=' + r.Dt__c.hashCode() + ';' + r.Dt__c.getTime() + ';' + System.hashCode(r.Dt__c);
                insert r;
                Dth_Probe__c rq = [SELECT Dt__c FROM Dth_Probe__c WHERE Id = :r.Id];
                p += '|FR=' + rq.Dt__c.hashCode() + ';' + rq.Dt__c.getTime() + ';' + System.hashCode(rq.Dt__c);
            } catch (Exception e) {
                p += '|DML=EX;' + e.getTypeName() + ';' + e.getMessage();
            }
            System.assert(false, p);
        }
    }
}
