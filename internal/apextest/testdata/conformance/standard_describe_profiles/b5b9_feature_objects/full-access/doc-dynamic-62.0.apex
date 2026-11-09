{ SObject r=new DocumentGenerationProcess(); Boolean ok=true; try{r.get('AdditionalMetadata');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentGenerationProcess.AdditionalMetadata');
ok=true; try{r.put('AdditionalMetadata',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentGenerationProcess.AdditionalMetadata');
ok=true; try{Database.query('SELECT AdditionalMetadata FROM DocumentGenerationProcess LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentGenerationProcess.AdditionalMetadata'); }
{ SObject r=new DocumentGenerationProcess(); Boolean ok=true; try{r.get('DocGenAdditionalInput');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentGenerationProcess.DocGenAdditionalInput');
ok=true; try{r.put('DocGenAdditionalInput',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentGenerationProcess.DocGenAdditionalInput');
ok=true; try{Database.query('SELECT DocGenAdditionalInput FROM DocumentGenerationProcess LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentGenerationProcess.DocGenAdditionalInput'); }
{ SObject r=new DocumentGenerationProcess(); Boolean ok=true; try{r.get('DocGenAdditionalInputType');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentGenerationProcess.DocGenAdditionalInputType');
ok=true; try{r.put('DocGenAdditionalInputType',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentGenerationProcess.DocGenAdditionalInputType');
ok=true; try{Database.query('SELECT DocGenAdditionalInputType FROM DocumentGenerationProcess LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentGenerationProcess.DocGenAdditionalInputType'); }
{ SObject r=new DocumentGenerationProcess(); Boolean ok=true; try{r.get('EstimatedDocumentSize');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentGenerationProcess.EstimatedDocumentSize');
ok=true; try{r.put('EstimatedDocumentSize',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentGenerationProcess.EstimatedDocumentSize');
ok=true; try{Database.query('SELECT EstimatedDocumentSize FROM DocumentGenerationProcess LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentGenerationProcess.EstimatedDocumentSize'); }
{ SObject r=new DocumentGenerationProcess(); Boolean ok=true; try{r.get('IsLocalizationEnabled');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentGenerationProcess.IsLocalizationEnabled');
ok=true; try{r.put('IsLocalizationEnabled',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentGenerationProcess.IsLocalizationEnabled');
ok=true; try{Database.query('SELECT IsLocalizationEnabled FROM DocumentGenerationProcess LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentGenerationProcess.IsLocalizationEnabled'); }
{ SObject r=new DocumentGenerationProcess(); Boolean ok=true; try{r.get('IsSharedExternally');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentGenerationProcess.IsSharedExternally');
ok=true; try{r.put('IsSharedExternally',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentGenerationProcess.IsSharedExternally');
ok=true; try{Database.query('SELECT IsSharedExternally FROM DocumentGenerationProcess LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentGenerationProcess.IsSharedExternally'); }
{ SObject r=new DocumentGenerationProcess(); Boolean ok=true; try{r.get('IsTocUpdateRequired');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentGenerationProcess.IsTocUpdateRequired');
ok=true; try{r.put('IsTocUpdateRequired',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentGenerationProcess.IsTocUpdateRequired');
ok=true; try{Database.query('SELECT IsTocUpdateRequired FROM DocumentGenerationProcess LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentGenerationProcess.IsTocUpdateRequired'); }
{ SObject r=new DocumentTemplate(); Boolean ok=true; try{r.get('ContextDefinitionName');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentTemplate.ContextDefinitionName');
ok=true; try{r.put('ContextDefinitionName',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentTemplate.ContextDefinitionName');
ok=true; try{Database.query('SELECT ContextDefinitionName FROM DocumentTemplate LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentTemplate.ContextDefinitionName'); }
{ SObject r=new DocumentTemplate(); Boolean ok=true; try{r.get('ContextFilterName');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentTemplate.ContextFilterName');
ok=true; try{r.put('ContextFilterName',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentTemplate.ContextFilterName');
ok=true; try{Database.query('SELECT ContextFilterName FROM DocumentTemplate LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentTemplate.ContextFilterName'); }
{ SObject r=new DocumentTemplate(); Boolean ok=true; try{r.get('ContextMappingName');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentTemplate.ContextMappingName');
ok=true; try{r.put('ContextMappingName',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentTemplate.ContextMappingName');
ok=true; try{Database.query('SELECT ContextMappingName FROM DocumentTemplate LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentTemplate.ContextMappingName'); }
{ SObject r=new DocumentTemplate(); Boolean ok=true; try{r.get('ContextTransformationName');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'get DocumentTemplate.ContextTransformationName');
ok=true; try{r.put('ContextTransformationName',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put DocumentTemplate.ContextTransformationName');
ok=true; try{Database.query('SELECT ContextTransformationName FROM DocumentTemplate LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(false,ok,'query DocumentTemplate.ContextTransformationName'); }