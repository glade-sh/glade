// Gives the running admin a role so it can own portal accounts (own transaction).
List<UserRole> rs = [SELECT Id FROM UserRole WHERE Name = 'C4 Admin Role'];
UserRole r;
if (rs.isEmpty()) { r = new UserRole(Name = 'C4 Admin Role'); insert r; } else { r = rs[0]; }
update new User(Id = UserInfo.getUserId(), UserRoleId = r.Id);
System.debug('CAP|p_role|ok');
