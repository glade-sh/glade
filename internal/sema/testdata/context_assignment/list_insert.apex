Database.SaveResult[] results = Database.insert(new List<Specimen__c>(), false); System.assertEquals(0, results.size());
