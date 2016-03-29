package trafcacc

import (
	"time"

	"github.com/Sirupsen/logrus"
)

type node struct {
	pool *streampool
	pqs  *packetQueue
}

func newNode() *node {
	return &node{
		pqs:  newPacketQueue(),
		pool: newStreamPool(),
	}
}

func (n *node) write(p *packet) error {
	return n.pool.write(p)
}

func (n *node) push(p *packet) {

	switch p.Cmd {
	case connected, connect:
		// TODO: maybe move d.pqs.create(p.Senderid, p.Connid) here?
	case closed, close:
		n.pqs.add(p)
		n.pool.cache.close(p.Senderid, p.Connid)
	case data: //data
		waiting := n.pqs.add(p)
		if waiting != 0 && waiting < p.Seqid {
			go func() {
				time.Sleep(time.Second / 10)
				waiting := n.pqs.waiting(p.Senderid, p.Connid)
				if waiting != 0 && waiting < p.Seqid {
					n.write(&packet{
						Senderid: p.Senderid,
						Connid:   p.Connid,
						Seqid:    waiting,
						Cmd:      rqu,
					})
					logrus.WithFields(logrus.Fields{
						"Connid":  p.Connid,
						"Seqid":   p.Seqid,
						"Waiting": waiting,
					}).Debugln("dialer send packet request")
				}
			}()
		}
	default:
		logrus.WithFields(logrus.Fields{
			"Cmd": p.Cmd,
		}).Warnln("unexpected Cmd in packet")
	}
}
