package pipeline
func runSync(job *Job,ctx *Context) {

	if job.onFailure != nil {
		defer job.onFailure.Process(ctx)
	}

	for _, task := range job.tasks {

		task.Process(ctx)
	}

	if job.onComplete != nil {
		for _,v:=range job.onComplete{
			v.Process(ctx)
		}
	}

}
