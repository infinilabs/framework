package pipeline

func run(job *Job,ctx *Context) {

	if job.sequential {
		runSync(job,ctx)
	} else {
		runAsync(job,ctx)
	}

}
