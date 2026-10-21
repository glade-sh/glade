global class Builder extends BuilderBase {
    global Builder() {}
    global List<String> insertRecords(Integer n) { return super.insertAll(n); }
    global String pkgOnly() { return 'pkgOnly'; }
    global String classApi() { return '62.0'; }
    global Builder self() { return this; }
    global List<String> direct(Integer n) { return insertRecords(n); }
    global List<String> chained(Integer n) { return self().insertRecords(n); }
    global List<String> thisQualified(Integer n) { return this.insertRecords(n); }
    global List<String> superQualified(Integer n) { return super.insertAll(n); }
}
