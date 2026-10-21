public class JsonSetterProbe {
  public static Integer constructorCalls = 0;
  public static Integer initializerCalls = 0;
  public static Integer setterCalls = 0;
  public static Integer getterCalls = 0;

  private static Map<String,String> initialValues() {
    initializerCalls++;
    return new Map<String,String>();
  }

  public class ConstructorDto {
    private Map<String,String> values;
    public ConstructorDto() {
      constructorCalls++;
      values = new Map<String,String>();
    }
    public Boolean mapIsNull() { return values == null; }
    public String Name {
      get { getterCalls++; return values.get('name'); }
      set { setterCalls++; values.put('name', value); }
    }
  }

  public class InitializerDto {
    private Map<String,String> values = initialValues();
    public InitializerDto() { constructorCalls++; }
    public Boolean mapIsNull() { return values == null; }
    public String Name {
      get { getterCalls++; return values.get('name'); }
      set { setterCalls++; values.put('name', value); }
    }
  }

  public virtual class ParentDto {
    private Map<String,String> values;
    protected ParentDto() {
      constructorCalls++;
      values = new Map<String,String>();
    }
    public Boolean mapIsNull() { return values == null; }
    protected String readValue() { getterCalls++; return values.get('name'); }
    protected void writeValue(String value) { setterCalls++; values.put('name', value); }
  }
  public class ChildDto extends ParentDto {
    public ChildDto() {}
    public String Name {
      get { return readValue(); }
      set { writeValue(value); }
    }
  }

  public static Map<String,Object> observe(String shape, String mode) {
    constructorCalls = 0; initializerCalls = 0; setterCalls = 0; getterCalls = 0;
    Map<String,Object> row = new Map<String,Object>{
      'shape'=>shape, 'mode'=>mode, 'decoded'=>false,
      'mapNull'=>null, 'read'=>null, 'errorType'=>null, 'errorStage'=>null
    };
    String stage = 'deserialize';
    try {
      Type target = shape == 'constructor' ? ConstructorDto.class :
        (shape == 'initializer' ? InitializerDto.class : ChildDto.class);
      Object decoded;
      if (mode == 'new') {
        decoded = shape == 'constructor' ? (Object)new ConstructorDto() :
          (shape == 'initializer' ? (Object)new InitializerDto() : (Object)new ChildDto());
      } else {
        decoded = mode == 'strict' ? JSON.deserializeStrict('{"Name":"Ada"}', target) :
          JSON.deserialize('{"Name":"Ada"}', target);
      }
      row.put('decoded', decoded != null);
      stage = 'read';
      if (shape == 'constructor') {
        ConstructorDto dto = (ConstructorDto)decoded;
        if (mode == 'new') dto.Name = 'Ada';
        row.put('mapNull', dto.mapIsNull()); row.put('read', dto.Name);
      } else if (shape == 'initializer') {
        InitializerDto dto = (InitializerDto)decoded;
        if (mode == 'new') dto.Name = 'Ada';
        row.put('mapNull', dto.mapIsNull()); row.put('read', dto.Name);
      } else {
        ChildDto dto = (ChildDto)decoded;
        if (mode == 'new') dto.Name = 'Ada';
        row.put('mapNull', dto.mapIsNull()); row.put('read', dto.Name);
      }
    } catch (Exception e) {
      row.put('errorType', e.getTypeName()); row.put('errorStage', stage);
    }
    row.put('constructors', constructorCalls); row.put('initializers', initializerCalls);
    row.put('setters', setterCalls); row.put('getters', getterCalls);
    return row;
  }
  public static List<Object> observeAll() {
    List<Object> rows = new List<Object>();
    for (String shape : new List<String>{'constructor','initializer','inherited'}) {
      for (String mode : new List<String>{'deserialize','strict','new'}) rows.add(observe(shape,mode));
    }
    return rows;
  }
}
