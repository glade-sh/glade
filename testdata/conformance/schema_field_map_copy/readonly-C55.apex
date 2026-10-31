Map<String,Schema.SObjectField> d=Schema.SObjectType.Account.fields.getMap();
d.putAll(new Map<String,Schema.SObjectField>{'addedkey'=>Contact.Name});
