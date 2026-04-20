package kafka

type WorkerPool struct {
	taskCh chan func()
}

func newWorkerPool(workerNum int, queueSize int) *WorkerPool {
	wp := &WorkerPool{
		taskCh: make(chan func(), queueSize),
	}

	for range workerNum {
		go func() {
			for task := range wp.taskCh {
				task()
			}
		}()
	}
	return wp
}

func (p *WorkerPool) submit(task func()) {
	select {
	case p.taskCh <- task:
		// 成功入队
	default:
		// 队列满了（必须处理）
	}
}
