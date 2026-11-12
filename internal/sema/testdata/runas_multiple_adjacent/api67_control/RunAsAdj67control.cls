@IsTest private class RunAsAdj67control { @IsTest static void probe() { User u=new User(Id=UserInfo.getUserId()); System.runAs(u) {} } }
