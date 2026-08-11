package coordinator

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestExchangeJSONCommitsCanonicalWorkflowResult(t *testing.T) {
	start := startTestCommand(t, "transport-start")
	stateRoot := t.TempDir()
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
	engine, err := NewEngine(startTestOptions(t, stateRoot, compiler))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(start)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := ExchangeJSON(bytes.NewReader(raw), &output, engine); err != nil {
		t.Fatal(err)
	}
	var result Result
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Kind != ResultState || result.WorkflowID == "" || result.Digest == "" {
		t.Fatalf("transport Result = %#v", result)
	}
}

func TestExchangeJSONRejectsInvalidCommandWithoutWriting(t *testing.T) {
	engine, err := NewEngine(Options{StateRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = ExchangeJSON(bytes.NewBufferString(`{"schema_version":"oaw.workflow-command/v1","kind":"START","unknown":true}`), &output, engine)
	if ErrorCode(err) != "WORKFLOW_COMMAND_DECODE_INVALID" || output.Len() != 0 {
		t.Fatalf("ExchangeJSON() = %v, output %q", err, output.String())
	}
}

func TestCoordinatorErrorUnwrapsCause(t *testing.T) {
	cause := errors.New("root cause")
	if (&Error{Code: "TEST", Cause: cause}).Unwrap() != cause {
		t.Fatal("Coordinator Error did not preserve its cause")
	}
}

func TestExchangeJSONReportsReaderAndWriterFailures(t *testing.T) {
	start := startTestCommand(t, "transport-io-failure")
	stateRoot := t.TempDir()
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
	engine, err := NewEngine(startTestOptions(t, stateRoot, compiler))
	if err != nil {
		t.Fatal(err)
	}
	if err := ExchangeJSON(transportErrorReader{}, &bytes.Buffer{}, engine); ErrorCode(err) != "WORKFLOW_COMMAND_READ_FAILED" {
		t.Fatalf("reader failure error = %v", err)
	}
	raw, err := json.Marshal(start)
	if err != nil {
		t.Fatal(err)
	}
	if err := ExchangeJSON(bytes.NewReader(raw), transportErrorWriter{}, engine); ErrorCode(err) != "WORKFLOW_RESULT_WRITE_FAILED" {
		t.Fatalf("writer failure error = %v", err)
	}
}

type transportErrorReader struct{}

func (transportErrorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

type transportErrorWriter struct{}

func (transportErrorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
