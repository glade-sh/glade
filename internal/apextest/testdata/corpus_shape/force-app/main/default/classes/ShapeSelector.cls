public virtual class ShapeSelector {
    public virtual Schema.SObjectType objectType() { return Account.SObjectType; }
    public virtual List<Schema.SObjectField> fields() {
        return new List<Schema.SObjectField>{Account.Id, Account.Name};
    }
    public virtual List<Account> selectByIds(Set<Id> ids) {
        return [SELECT Id, Name FROM Account WHERE Id IN :ids];
    }
}
