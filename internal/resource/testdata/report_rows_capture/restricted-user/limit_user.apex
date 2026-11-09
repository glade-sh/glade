Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1]; User u=[SELECT Id,ProfileId FROM User WHERE Alias='b6usr' LIMIT 1]; u.ProfileId=p.Id;update u;
