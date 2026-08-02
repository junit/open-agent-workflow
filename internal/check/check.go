package check

import (
	"io"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/management"
)

type Environment = management.Environment
type Request = management.CheckRequest
type Result = management.Result
type Error = management.Error

func Execute(value catalog.Catalog, environment Environment, request Request) (Result, error) {
	return management.Check(value, environment, request)
}

func Write(result Result, output io.Writer) error {
	return management.WriteResult(result, output)
}
