package pipeline

// Dag represents directed acyclic graph
type Dag struct {
	mode string
	jobs []*Job
}

// New creates new DAG
func NewDAG(mode string) *Dag {
	return &Dag{
		mode: mode,
		jobs: make([]*Job, 0),
	}
}

func (dag *Dag) lastJob() *Job {
	jobsCount := len(dag.jobs)
	if jobsCount == 0 {
		return nil
	}

	return dag.jobs[jobsCount-1]
}

func (dag *Dag) Parse(dsl string) *Dag {
	return dag
}

// Run starts the tasks
// It will block until all functions are done
func (dag *Dag) Run(ctx *Context) {

	//fmt.Println("total jobs:",len(dag.jobs))
	for _, job := range dag.jobs {
		run(job,ctx)
	}

}

// RunAsync executes Run on another goroutine
func (dag *Dag) RunAsync(ctx *Context,onComplete func()) {
	go func() {

		dag.Run(ctx)

		if onComplete != nil {
			onComplete()
		}

	}()
}

// Pipeline executes tasks sequentially
func (dag *Dag) Pipeline(tasks ...Processor) *pipelineResult {

	job := &Job{
		tasks:      make([]Processor, len(tasks)),
		sequential: true,
	}

	for i, task := range tasks {
		job.tasks[i] = task
	}

	dag.jobs = append(dag.jobs, job)

	return &pipelineResult{
		dag,
	}
}

func (dag *Dag) Spawns(tasks ...Processor) *spawnsResult {

	job := &Job{
		tasks:      make([]Processor, len(tasks)),
		sequential: false,
		mode:dag.mode,
	}

	for i, task := range tasks {
		job.tasks[i] = task
	}

	dag.jobs = append(dag.jobs, job)

	return &spawnsResult{
		dag,
	}
}

type anyResult struct {
	dag *Dag
}

func (dag *Dag) Any(tasks ...func()) *anyResult {
	return &anyResult{
		dag,
	}
}
