@IsTest private class RunAsAdj62control { @IsTest static void probe() { User u=new User(Id=UserInfo.getUserId()); System.runAs(u) {} } }
