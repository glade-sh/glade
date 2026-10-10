package vm

import "testing"

func TestUnitOfWorkCustomDMLProcessesEveryRecycleBinBucket(t *testing.T) {
	noop, err := CompileAnonymous("return;")
	if err != nil {
		t.Fatal(err)
	}
	recordCount, err := CompileAnonymous("this.recycled = this.recycled + records.size(); this.order = this.order + records.get(0).Id;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "UOWCustomDML",
		Interfaces: []string{"framework_SObjectUnitOfWork.IDML"},
		Methods: map[string]Method{
			"dmlInsert":       {Name: "UOWCustomDML.dmlInsert", ClassName: "UOWCustomDML", ReturnType: "void", Params: []Param{{Name: "records", Type: "List<SObject>"}}, Program: noop},
			"dmlUpdate":       {Name: "UOWCustomDML.dmlUpdate", ClassName: "UOWCustomDML", ReturnType: "void", Params: []Param{{Name: "records", Type: "List<SObject>"}}, Program: noop},
			"dmlDelete":       {Name: "UOWCustomDML.dmlDelete", ClassName: "UOWCustomDML", ReturnType: "void", Params: []Param{{Name: "records", Type: "List<SObject>"}}, Program: noop},
			"eventPublish":    {Name: "UOWCustomDML.eventPublish", ClassName: "UOWCustomDML", ReturnType: "void", Params: []Param{{Name: "records", Type: "List<SObject>"}}, Program: noop},
			"emptyRecycleBin": {Name: "UOWCustomDML.emptyRecycleBin", ClassName: "UOWCustomDML", ReturnType: "void", Params: []Param{{Name: "records", Type: "List<SObject>"}}, Program: recordCount},
		},
	}); err != nil {
		t.Fatal(err)
	}
	dml := Object("UOWCustomDML")
	dml.Fields["recycled"] = Int(0)
	dml.Fields["order"] = String("")
	uow, err := machine.constructFrameworkSObjectUnitOfWork([]Value{
		List(sObjectTypeToken("Product2"), sObjectTypeToken("Opportunity")),
		dml,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	product := Object("Product2")
	product.Fields["Id"] = platformScalar("Id", "01t000000000002")
	opportunity := Object("Opportunity")
	opportunity.Fields["Id"] = platformScalar("Id", "006000000000001")
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registerpermanentlydeleted", []Value{List(opportunity, product)}, &Result{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "commitwork", nil, &Result{}); err != nil {
		t.Fatal(err)
	}
	if got := dml.Fields["recycled"].Int; got != 2 {
		t.Fatalf("custom emptyRecycleBin record count = %d, want 2", got)
	}
	if got := dml.Fields["order"].Text; got != "006000000000001AAA01t000000000002AAA" {
		t.Fatalf("custom emptyRecycleBin order = %q, want Opportunity then Product2", got)
	}
}
