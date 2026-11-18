@IsTest private class RunAsAdj67variables { @IsTest static void probe() { User u=new User(Id=UserInfo.getUserId()); Version v=new Version(1,0); System.runAs(u,v) {} } }
