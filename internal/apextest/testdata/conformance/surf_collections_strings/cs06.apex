List<String> rows = new List<String>();
try { rows.add('sq=' + String.escapeSingleQuotes('O\'Reilly \\ x')); } catch (Exception e) { rows.add('sq=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('csv=' + 'a,b"c\nd'.escapeCsv()); } catch (Exception e) { rows.add('csv=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('csvCr=' + 'a\rb'.escapeCsv()); } catch (Exception e) { rows.add('csvCr=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('csvSpace=' + ' a '.escapeCsv()); } catch (Exception e) { rows.add('csvSpace=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('csvEmpty=[' + ''.escapeCsv() + ']'); } catch (Exception e) { rows.add('csvEmpty=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('html=' + '<a href="x">&\'</a>'.escapeHtml4()); } catch (Exception e) { rows.add('html=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('htmlGt=' + 'a>b'.escapeHtml4()); } catch (Exception e) { rows.add('htmlGt=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('htmlEnt=' + '&amp;'.escapeHtml4()); } catch (Exception e) { rows.add('htmlEnt=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('ab5=' + 'abcdefgh'.abbreviate(5)); } catch (Exception e) { rows.add('ab5=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('ab2=' + 'abcdefgh'.abbreviate(2)); } catch (Exception e) { rows.add('ab2=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('ab52=' + 'abcdefgh'.abbreviate(5, 2)); } catch (Exception e) { rows.add('ab52=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('sct=' + JSON.serialize('fooBarBaz123_x'.splitByCharacterType())); } catch (Exception e) { rows.add('sct=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('sctCamel=' + JSON.serialize('a1_b2Xy'.splitByCharacterTypeCamelCase())); } catch (Exception e) { rows.add('sctCamel=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('sctU=' + JSON.serialize('éA1_'.splitByCharacterType())); } catch (Exception e) { rows.add('sctU=' + e.getTypeName() + '|' + e.getMessage()); }
System.debug('OBS|' + String.join(rows, ';'));
