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
}

func (q *Queue) dequeue() url.Values {
	q.lock()
	defer q.unlock()
	if len(q.values) == 0 {
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
