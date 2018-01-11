package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

type node struct {
	id     string
	el     interface{}
	next   *node
	inChan <-chan interface{}
	lock   *sync.Mutex
	wg     *sync.WaitGroup
	size   int
}

func (n *node) start() {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		for {
			select {
			case el := <-n.inChan:
				n.lock.Lock()
				n.push(el)
				n.lock.Unlock()
			}
		}
	}()
}

func (n *node) push(el interface{}) {
	prev := *n
	nn := node{
		el:     el,
		next:   &prev,
		lock:   prev.lock,
		inChan: prev.inChan,
		id:     prev.id,
		wg:     prev.wg,
		size:   prev.size + 1,
	}

	*n = nn
}

func (n *node) pull() interface{} {
	next := n.next
	el := n.el
	if next.next == nil {
		next.next = &node{}
	}

	*n = *next

	return el
}

func (n *node) pullBulk(i int) []interface{} {
	bulk := []interface{}{}
	el := n.pull()
	for i > 0 && el != nil {
		bulk = append(bulk, el)
		el = n.pull()
		i--
	}
	return bulk
}

func (n node) String() string {
	end := "nil"
	var list string
	next := &n

	if next.el == nil {
		return end
	}

	for next != nil {
		list = fmt.Sprintf("%s%d -> ", list, next.el)
		next = next.next
	}

	return list + end
}

type queue struct {
	name   string
	nodes  []chan<- interface{}
	fnodes []*node
	wg     *sync.WaitGroup
	lock   *sync.Mutex
	rand   *rand.Rand
}

func (q *queue) push(el interface{}) {
	next := q.rand.Intn(len(q.nodes))
	q.nodes[next] <- el
}

func (q *queue) size() int {
	var s int
	for _, n := range q.fnodes {
		s += n.size
	}
	return s
}

func newQueue(name string, c int, wg *sync.WaitGroup) queue {
	nodes := make([]chan<- interface{}, c)
	fnodes := make([]*node, c)
	s := rand.NewSource(time.Now().UnixNano())
	rand := rand.New(s)
	for i := 0; i < c; i++ {
		ch := make(chan interface{}, 2)
		n := node{
			id:     fmt.Sprint(name, i),
			inChan: ch,
			lock:   &sync.Mutex{},
			wg:     wg,
		}
		n.start()
		nodes[i] = ch
		fnodes[i] = &n
	}
	return queue{
		name:   name,
		nodes:  nodes,
		rand:   rand,
		fnodes: fnodes,
	}
}

func run(c int) {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	var wg sync.WaitGroup
	m := sync.Mutex{}
	var n int

	q := newQueue("test", c, &wg)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-time.After(time.Duration(100) * time.Microsecond):
					el := r1.Intn(1000)
					q.push(el)
					m.Lock()
					n++
					m.Unlock()
				}
			}

		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-time.After(5 * time.Second):
				log.Printf("sent: %d, recived: %d", n, q.size())
			}
		}
	}()

	<-time.After(10 * time.Second)
	fmt.Println(n)
	fmt.Println(q.size())
	fmt.Println(q.fnodes[0])
}

func main() {
	run(1)
}
