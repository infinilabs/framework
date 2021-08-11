package pipeline

// Job - Each job consists of one or more tasks
// Each Job can runs tasks in order(Sequential) or unordered
type Job struct {
	tasks      []Processor
	sequential bool
	mode       string
	onComplete []Processor
	onFailure  Processor
}
