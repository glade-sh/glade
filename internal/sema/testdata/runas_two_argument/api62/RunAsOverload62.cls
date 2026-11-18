@IsTest private class RunAsOverload62 { @IsTest static void probe() { System.runAs(new User(Id=UserInfo.getUserId()),new Version(1,0)) { System.assert(true); } } }
