// Compile-only: System enum constants beside a project class that shadows a platform enum name.
System.assert(System.LoggingLevel.ERROR.name() == 'ERROR' && System.LoggingLevel.WARN.ordinal() >= 0);
System.assert(ShapeLevelShadow.compileRoutes().size() == 3);
System.assert(Quiddity.ANONYMOUS.name() + System.Quiddity.ANONYMOUS.ordinal() + JSONToken.START_OBJECT.name() + System.JSONToken.END_OBJECT.ordinal() + DisplayType.STRING.name() + Schema.DisplayType.BOOLEAN.ordinal() != null);
System.assert(ShapeSystemEnums.compileRoutes() != null);
// Compile-only: Schema-qualified standard and setup objects in anonymous Apex.
// Native basis: sema/testdata/schema_relationship_gate/org.tsv:301,307,319,325.
Schema.Contact c = new Schema.Contact();
Schema.Account a = new Schema.Account(Name = 'Shape');
Schema.Group g = new Schema.Group(Name = 'Shape', Type = 'Queue');
Schema.User u = new Schema.User(LastName = 'Shape');
List<Schema.Group> gs = new List<Schema.Group>{ g };
Map<Id, Schema.Group> byId = new Map<Id, Schema.Group>();
Map<Id, Schema.User> users = new Map<Id, Schema.User>();
System.assert(c.Account == null);
System.assert(a.Name == 'Shape');
System.assert(gs.size() == 1 && byId.isEmpty() && users.isEmpty() && u.LastName == 'Shape');
System.assert(new ShapeSchemaTypes().observe());
Map<String, Schema.SObjectField> fieldMap = SObjectType.Account.fields.getMap();
Schema.DescribeFieldResult direct = SObjectType.Account.fields.Name;
Map<String, Schema.FieldSet> fieldSets = SObjectType.Account.fieldSets.getMap();
System.assert(fieldMap.containsKey('name') && direct != null && fieldSets != null);
System.assert(ShapeShortFields.compileRoutes());
Map<String, Schema.SObjectField> customMap = SObjectType.ShapeLeaf__c.fields.getMap();
System.assert(customMap != null && SObjectType.ShapeLeaf__c.fields.Grove__c.getName() != null);
