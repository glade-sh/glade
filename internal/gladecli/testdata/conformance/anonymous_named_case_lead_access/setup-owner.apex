UserRole r = new UserRole(Name='Sharing Probe Owner'); insert r;
update new User(Id=UserInfo.getUserId(), UserRoleId=r.Id);
