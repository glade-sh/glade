public virtual class PropertySurfaceTarget {
 public String label {get {return value();}}
 public String plain {get;set;}
 public String fieldValue;
 public virtual String value() {return 'declared';}
}
