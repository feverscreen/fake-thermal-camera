package fakecamera

import (
	"net/url"
	"sync"
)

const (
	capacity = 3
)

type Queue struct {
	values   []url.Values
	pending  bool
	waitCond *sync.Cond
}

func newQueue() *Queue {
	return &Queue{values: make([]url.Values, 0, capacity), waitCond: sync.NewCond(&sync.Mutex{})}
}

func (q *Queue) lock() {
	q.waitCond.L.Lock()
}

func (q *Queue) unlock() {
	q.waitCond.L.Unlock()
}

func (q *Queue) enqueue(params url.Values) {
	q.lock()
	defer q.unlock()
	q.values = append(q.values, params)
	if q.pending {
		q.waitCond.Signal()
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
	q.lock()
	defer q.unlock()
	if !q.pending {
		return
	}
	q.waitCond.Wait()

}
