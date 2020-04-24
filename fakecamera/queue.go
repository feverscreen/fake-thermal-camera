package fakecamera

import (
	"net/url"
	"sync"
)

const (
	capacity = 3
)

type Queue struct {
	queueLock sync.Mutex
	values    []url.Values
	pending   bool
	wg        sync.WaitGroup
}

func newQueue() *Queue {
	return &Queue{values: make([]url.Values, 0, capacity)}
}

func (q *Queue) lock() {
	q.queueLock.Lock()
}

func (q *Queue) unlock() {
	q.queueLock.Unlock()
}

func (q *Queue) enqueue(params url.Values) {
	q.lock()
	defer q.unlock()
	q.values = append(q.values, params)
	if q.pending {
		q.wg.Done()
		q.pending = false
	}
}

func (q *Queue) dequeue() url.Values {
	q.lock()
	defer q.unlock()
	if len(q.values) == 0 {
		q.pending = true
		return nil
	}
	top := q.values[0]
	q.values = q.values[1:]
	return top
}

func (q *Queue) clear() {
	q.lock()
	defer q.unlock()
	q.values = make([]url.Values, 0, capacity)
}

func (q *Queue) wait() {
	if !q.pending {
		return
	}
	q.wg.Add(1)
	q.wg.Wait()

}