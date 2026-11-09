{ SObject r=new GiftCommitment(); Boolean ok=true; try{r.get('LastNextGenCmtProcDtTm');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'get GiftCommitment.LastNextGenCmtProcDtTm');
ok=true; try{r.put('LastNextGenCmtProcDtTm',null);}catch(Exception e){ok=false;} System.assertEquals(true,ok,'put GiftCommitment.LastNextGenCmtProcDtTm');
ok=true; try{Database.query('SELECT LastNextGenCmtProcDtTm FROM GiftCommitment LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'query GiftCommitment.LastNextGenCmtProcDtTm'); }
{ SObject r=new GiftCommitment(); Boolean ok=true; try{r.get('LastNextGenCmtProcError');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'get GiftCommitment.LastNextGenCmtProcError');
ok=true; try{r.put('LastNextGenCmtProcError',null);}catch(Exception e){ok=false;} System.assertEquals(true,ok,'put GiftCommitment.LastNextGenCmtProcError');
ok=true; try{Database.query('SELECT LastNextGenCmtProcError FROM GiftCommitment LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'query GiftCommitment.LastNextGenCmtProcError'); }
{ SObject r=new GiftCommitment(); Boolean ok=true; try{r.get('PartyPhilanthropicRsrchPrflId');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'get GiftCommitment.PartyPhilanthropicRsrchPrflId');
ok=true; try{r.put('PartyPhilanthropicRsrchPrflId',null);}catch(Exception e){ok=false;} System.assertEquals(true,ok,'put GiftCommitment.PartyPhilanthropicRsrchPrflId');
ok=true; try{Database.query('SELECT PartyPhilanthropicRsrchPrflId FROM GiftCommitment LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'query GiftCommitment.PartyPhilanthropicRsrchPrflId'); }
{ SObject r=new GiftTransaction(); Boolean ok=true; try{r.get('GenerationalCohort');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'get GiftTransaction.GenerationalCohort');
ok=true; try{r.put('GenerationalCohort',null);}catch(Exception e){ok=false;} System.assertEquals(true,ok,'put GiftTransaction.GenerationalCohort');
ok=true; try{Database.query('SELECT GenerationalCohort FROM GiftTransaction LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'query GiftTransaction.GenerationalCohort'); }
{ SObject r=new GiftTransaction(); Boolean ok=true; try{r.get('PartyPhilanthropicRsrchPrflId');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'get GiftTransaction.PartyPhilanthropicRsrchPrflId');
ok=true; try{r.put('PartyPhilanthropicRsrchPrflId',null);}catch(Exception e){ok=false;} System.assertEquals(true,ok,'put GiftTransaction.PartyPhilanthropicRsrchPrflId');
ok=true; try{Database.query('SELECT PartyPhilanthropicRsrchPrflId FROM GiftTransaction LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'query GiftTransaction.PartyPhilanthropicRsrchPrflId'); }
{ SObject r=new GiftTransaction(); Boolean ok=true; try{r.get('TaxDeductionAmount');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'get GiftTransaction.TaxDeductionAmount');
ok=true; try{r.put('TaxDeductionAmount',null);}catch(Exception e){ok=false;} System.assertEquals(false,ok,'put GiftTransaction.TaxDeductionAmount');
ok=true; try{Database.query('SELECT TaxDeductionAmount FROM GiftTransaction LIMIT 1');}catch(Exception e){ok=false;} System.assertEquals(true,ok,'query GiftTransaction.TaxDeductionAmount'); }