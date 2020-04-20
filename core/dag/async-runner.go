package dag

import "sync"

func runAsync(job *Job) {

	//if job.onFailure != nil {
	//	defer job.onFailure()
	//}

	wg := &sync.WaitGroup{}
	wg.Add(len(job.tasks))

	for _, task := range job.tasks {
		go func(task func()) {
			task()
			wg.Done()
		}(task)
	}

	wg.Wait()
	if job.onComplete != nil {
		job.onComplete()
	}
}
