// Compile-only: System enum constants beside a project class that shadows a platform enum name.
System.assert(System.LoggingLevel.ERROR.name() == 'ERROR' && System.LoggingLevel.WARN.ordinal() >= 0);
System.assert(ShapeLevelShadow.compileRoutes().size() == 3);
System.assert(Quiddity.ANONYMOUS.name() + System.Quiddity.ANONYMOUS.ordinal() + JSONToken.START_OBJECT.name() + System.JSONToken.END_OBJECT.ordinal() + DisplayType.STRING.name() + Schema.DisplayType.BOOLEAN.ordinal() != null);
System.assert(ShapeSystemEnums.compileRoutes() != null);
