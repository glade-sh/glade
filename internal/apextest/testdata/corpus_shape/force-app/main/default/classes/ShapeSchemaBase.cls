public virtual class ShapeSchemaBase {
    // Compile-only: Schema-qualified standard and setup objects as inherited field types.
    // Native basis: sema/testdata/schema_relationship_gate/org.tsv:301,307,319,325.
    public Schema.Contact item = new Schema.Contact();
    protected Schema.Account parent;
    protected Schema.Group baseGroup = new Schema.Group();
    protected List<Schema.User> baseUsers = new List<Schema.User>();
}
