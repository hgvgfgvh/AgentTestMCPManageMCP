package engine_test

import (
	"context"
	"encoding/json"
	"testing"

	"AgentTestMCPManageMCP/internal/engine"
	"AgentTestMCPManageMCP/internal/response"
)

func TestStub_ListAddExecute(t *testing.T) {
	eng := engine.NewStubEngine()
	ctx := context.Background()

	listJSON := eng.ListManaged(ctx, engine.ListInput{})
	var list response.ListPayload
	if err := json.Unmarshal([]byte(listJSON), &list); err != nil {
		t.Fatal(err)
	}
	if list.CatalogRevision < 1 || len(list.Managed) < 1 {
		t.Fatalf("list: %+v", list)
	}
	rev0 := list.CatalogRevision

	addJSON := eng.AddManaged(ctx, engine.AddInput{Requirement: "需要 SQLite 只读"})
	if key, bad := response.AddResponseHasForbiddenFields(addJSON); bad {
		t.Fatalf("add must not contain %q: %s", key, addJSON)
	}
	var add response.AddPayload
	if err := json.Unmarshal([]byte(addJSON), &add); err != nil {
		t.Fatal(err)
	}
	if !add.Accepted || add.MCPID == "" || add.Status != "ready" {
		t.Fatalf("add: %+v", add)
	}

	listJSON2 := eng.ListManaged(ctx, engine.ListInput{})
	var list2 response.ListPayload
	_ = json.Unmarshal([]byte(listJSON2), &list2)
	if list2.CatalogRevision <= rev0 {
		t.Fatalf("revision should increase: %d -> %d", rev0, list2.CatalogRevision)
	}

	execJSON := eng.ExecuteStep(ctx, engine.ExecuteInput{
		MCPID:    add.MCPID,
		StepGoal: "查询一行",
		Rawdata:  `{"sql":"SELECT 1"}`,
	})
	var ex response.ExecutePayload
	if err := json.Unmarshal([]byte(execJSON), &ex); err != nil {
		t.Fatal(err)
	}
	if !ex.OK || ex.Payload == "" || ex.ToolUsed == "" {
		t.Fatalf("execute: %+v", ex)
	}
}

func TestStub_AddRejectsEmptyRequirement(t *testing.T) {
	eng := engine.NewStubEngine()
	out := eng.AddManaged(context.Background(), engine.AddInput{})
	var add response.AddPayload
	_ = json.Unmarshal([]byte(out), &add)
	if add.Accepted || add.Status != "failed" {
		t.Fatalf("expected failure: %s", out)
	}
}
