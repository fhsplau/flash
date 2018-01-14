package main

import (
	"context"
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
	size   int
	ctx    context.Context
}

func (n *node) start() {
	go func() {
		for {
			select {
			case el := <-n.inChan:
				tmp := n.el
				n.lock.Lock()
				n.push(el)
				if n.next.el != tmp {
					log.Fatal("something went wrong!")
				}
				n.lock.Unlock()
			case <-n.ctx.Done():
				return
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
		size:   prev.size + 1,
		ctx:    prev.ctx,
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
	next := &n

	if next.el == nil {
		return end
	}

	for next != nil {
		fmt.Printf("%d -> ", next.el)
		next = next.next
	}
	fmt.Println(end)

	return ""
}

type queue struct {
	name   string
	nodes  []chan<- interface{}
	fnodes []*node
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

func newQueue(ctx context.Context, name string, c int) queue {
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
			ctx:    ctx,
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

func run(c int, duration time.Duration) {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	m := sync.Mutex{}
	var n int
	ctx, f := context.WithTimeout(context.Background(), duration)
	defer f()

	q := newQueue(ctx, "test", c)

	for i := 0; i < 3; i++ {
		go func() {
			for {
				select {
				case <-time.After(time.Duration(100) * time.Microsecond):
					el := r1.Intn(1000)
					q.push(el)
					m.Lock()
					n++
					m.Unlock()
				case <-ctx.Done():
					return
				}
			}

		}()
	}

	go func() {
		for {
			select {
			case <-time.After(5 * time.Second):
				log.Printf("sent: %d, recived: %d", n, q.size())
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	log.Printf("sent: %d, recived: %d", n, q.size())
}

func main() {
	run(1, time.Duration(20)*time.Second)
}
