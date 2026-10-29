public class LogicProbe {
    static Map<String, Object> show(Account a) {
        Account r = [SELECT Site, AccountNumber, Fax, Phone, Description, NumberOfEmployees, AnnualRevenue FROM Account WHERE Id = :a.Id];
        return new Map<String, Object>{ 'site' => r.Site, 'accountNumber' => r.AccountNumber, 'fax' => r.Fax, 'phone' => r.Phone, 'description' => r.Description, 'employees' => r.NumberOfEmployees, 'revenue' => r.AnnualRevenue };
    }
    static Map<String, Object> put(Account a) {
        try { insert a; } catch (Exception e) { return new Map<String, Object>{ 'exception' => e.getTypeName() }; }
        return show(a);
    }
    public static Map<String, Object> observe() {
        Map<String, Object> m = new Map<String, Object>();
        m.put('E01', put(new Account(Name = 'Ent1', Industry = 'Energy')));
        m.put('E02', put(new Account(Name = 'Ent2', Rating = 'Hot')));
        m.put('E03', put(new Account(Name = 'Ent3', Industry = 'Banking', Rating = 'Cold')));
        m.put('E04', put(new Account(Name = 'Ent4', Industry = 'Energy', Rating = 'Hot')));
        m.put('E05', put(new Account(Name = 'Ent5', NumberOfEmployees = 200)));
        m.put('E06', put(new Account(Name = 'Ent6', Rating = 'Hot', NumberOfEmployees = 200)));
        m.put('E07', put(new Account(Name = 'Ent7', Industry = 'Energy', NumberOfEmployees = 200)));
        m.put('E08', put(new Account(Name = 'Zed8', NumberOfEmployees = 50)));
        m.put('E09', put(new Account(Name = 'Ent9', NumberOfEmployees = 3)));
        m.put('E10', put(new Account(Name = 'Ent10')));
        m.put('L01', put(new Account(Name = 'LOOPTRIG')));
        insert new Account[]{ new Account(Name = 'LoopSeedA', NumberOfEmployees = 3, AnnualRevenue = 10.25), new Account(Name = 'LoopSeedB', NumberOfEmployees = 4, AnnualRevenue = 5.5), new Account(Name = 'LoopSeedC') };
        Account t = new Account(Name = 'LOOPTRIG');
        m.put('L02', put(t));
        m.put('L03', put(new Account(Name = 'LOOPCUR')));
        m.put('L04', put(new Account(Name = 'LOOPCUR2')));
        return m;
    }
}
