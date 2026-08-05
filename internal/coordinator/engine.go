package coordinator

type Engine struct{}

func (engine *Engine) Exchange(Command) (Result, error) {
	if engine == nil {
		return Result{}, coordinatorError("WORKFLOW_ENGINE_UNAVAILABLE", "Workflow Engine is required", nil)
	}
	return Result{}, coordinatorError("WORKFLOW_ENGINE_UNAVAILABLE", "Workflow Engine state is not initialized", nil)
}
