package pipeline

type pipelineResult struct {
	dag *Dag
}

func (result *pipelineResult) Then() *pipelineDSL {
	return &pipelineDSL{
		result.dag,
	}
}

func (result *pipelineResult) OnComplete(action ...Processor) *pipelineResult {
	job := result.dag.lastJob()
	if job != nil {
		job.onComplete = action
	}
	return result
}

func (result *pipelineResult) OnFailure(action Processor) *pipelineResult {
	job := result.dag.lastJob()
	if job != nil {
		job.onFailure = action
	}
	return result
}

type pipelineDSL struct {
	dag *Dag
}

func (dsl *pipelineDSL) Spawns(tasks ...Processor) *spawnsResult {
	dsl.dag.Spawns(tasks...)
	return &spawnsResult{
		dsl.dag,
	}
}

type spawnsResult struct {
	dag *Dag
}

func (result *spawnsResult) Join() *spawnsDSL {
	return &spawnsDSL{
		result.dag,
	}
}

func (result *spawnsResult) OnComplete(action ...Processor) *spawnsResult {
	job := result.dag.lastJob()
	if job != nil {
		job.onComplete = action
	}
	return result
}

type spawnsDSL struct {
	dag *Dag
}

func (dsl *spawnsDSL) Pipeline(tasks ...Processor) *pipelineResult {
	dsl.dag.Pipeline(tasks...)
	return &pipelineResult{
		dsl.dag,
	}
}
