package dag

func runSync(job *Job) {

	//if job.onFailure != nil {
	//	defer job.onFailure()
	//}

	for _, task := range job.tasks {
		task()
	}
	if job.onComplete != nil {
		job.onComplete()
	}

}
