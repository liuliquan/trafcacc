package trafcacc

import (
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

type cmd uint8

const (
	data cmd = iota
	close
	closed
	connect
	connected
	ping
	pong
)

type packet struct {
	Senderid uint32
	Connid   uint32
	Seqid    uint32
	Buf      []byte
	Cmd      cmd
}

type queue struct {
	queue        map[uint32]*packet
	cond         *sync.Cond
	waitingSeqid uint32
}

type packetQueue struct {
	queue  map[uint32]map[uint32]*queue
	closed map[uint32]map[uint32]struct{}
	mux    sync.Mutex
}

func newPacketQueue() *packetQueue {
	return &packetQueue{
		queue:  make(map[uint32]map[uint32]*queue),
		closed: make(map[uint32]map[uint32]struct{}),
	}
}

func (pq *packetQueue) create(senderid, connid uint32) (isnew bool) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	if _, ok := pq.queue[senderid]; !ok {
		pq.queue[senderid] = make(map[uint32]*queue)
		pq.closed[senderid] = make(map[uint32]struct{})
	}

	if _, ok := pq.queue[senderid][connid]; !ok {
		delete(pq.closed[senderid], connid)

		pq.queue[senderid][connid] = &queue{
			queue:        make(map[uint32]*packet),
			cond:         sync.NewCond(&sync.Mutex{}),
			waitingSeqid: 1,
		}
		return true
	}
	return false
}

func (pq *packetQueue) close(senderid, connid uint32) {
	// TODO: wait queue drained cleanedup
	pq.mux.Lock()
	if q, ok := pq.queue[senderid][connid]; ok {
		delete(pq.queue[senderid], connid)
		pq.mux.Unlock()

		cond := q.cond
		cond.L.Lock()
		pq.mux.Lock()
		pq.closed[senderid][connid] = struct{}{}
		pq.mux.Unlock()
		cond.L.Unlock()
		cond.Broadcast()

		go func() {
			// expired after 30 minutes
			<-time.After(30 * time.Minute)

			cond.L.Lock()
			if _, ok := pq.closed[senderid][connid]; ok {
				delete(pq.closed[senderid], connid)
			}
			cond.L.Unlock()
			cond.Broadcast()
		}()
	} else {
		pq.mux.Unlock()
	}
}

func (pq *packetQueue) add(p *packet) {

	pq.mux.Lock()
	_, ok := pq.closed[p.Senderid][p.Connid]
	pq.mux.Unlock()
	if !ok {

		pq.mux.Lock()
		q, ok := pq.queue[p.Senderid][p.Connid]
		pq.mux.Unlock()
		if ok {

			// TODO: lock
			q.cond.L.Lock()
			if _, ok := q.queue[p.Seqid]; !ok {

				q.queue[p.Seqid] = p

				q.cond.L.Unlock()
				q.cond.Broadcast()
			} else { // else drop duplicated packet
				q.cond.L.Unlock()
			}

		} else {
			logrus.WithFields(logrus.Fields{
				"packet": p,
			}).Fatalln("packetQueue havn't been created")
		}
	}
}

func (pq *packetQueue) arrived(senderid, connid uint32) bool {
	if pq.isclosed(senderid, connid) {
		return true // if connection closed
	}
	pq.mux.Lock()
	q, ok := pq.queue[senderid][connid]
	pq.mux.Unlock()
	if ok {
		if _, ok := q.queue[q.waitingSeqid]; ok {
			return true
		}

	}
	return false
}

func (pq *packetQueue) waitforArrived(senderid, connid uint32) {
	cond := pq.cond(senderid, connid)
	cond.L.Lock()
	if !pq.arrived(senderid, connid) {
		cond.Wait()
	}
	cond.L.Unlock()
}

func (pq *packetQueue) isclosed(senderid, connid uint32) bool {
	if _, ok := pq.closed[senderid][connid]; ok {
		return true // connection closed
	}
	return false
}

func (pq *packetQueue) pop(senderid, connid uint32) *packet {
	if q, ok := pq.queue[senderid][connid]; ok {
		q.cond.L.Lock()
		if p, ok := q.queue[q.waitingSeqid]; ok {
			defer func() {
				q.cond.L.Unlock()
				q.cond.Broadcast()
			}()
			delete(q.queue, q.waitingSeqid)
			q.waitingSeqid++
			return p
		}
		q.cond.L.Unlock()
	}
	return nil
}

func (pq *packetQueue) cond(senderid, connid uint32) *sync.Cond {
	pq.mux.Lock()
	defer pq.mux.Unlock()
	if q, ok := pq.queue[senderid][connid]; ok {
		return q.cond
	}
	return nil
}