// debug.go 辅助 Debug 的一些 helper function

package trafcacc

import (
	"runtime"
	"sync"

	log "github.com/Sirupsen/logrus"
)

var (
	routineList = make(map[string]int)
	routineMux  = &sync.RWMutex{}
)

type Trafcacc interface {
	PrintStatus()
}

func (t *trafcacc) PrintStatus() {
	s := new(runtime.MemStats)
	runtime.ReadMemStats(s)
	routineMux.RLock()
	totalGoroutineTracked := 0
	for _, v := range routineList {
		totalGoroutineTracked += v
	}

	log.WithFields(log.Fields{
		"NumGoroutine":     runtime.NumGoroutine(),
		"Alloc":            s.Alloc,
		"HeapObjects":      s.HeapObjects,
		"TrackedGoroutine": totalGoroutineTracked,
		"Detail":           routineList,
	}).Infoln(t.roleString(), "status")

	routineMux.RUnlock()
}

func routineAdd(name string) {
	routineMux.Lock()
	routineList[name] = routineList[name] + 1
	routineMux.Unlock()
}

func routineDel(name string) {
	routineMux.Lock()
	routineList[name] = routineList[name] - 1
	routineMux.Unlock()
}

func keysOfmap(m map[uint32]*packet) []uint32 {
	rlen := len(m)
	if rlen > 10 {
		rlen = 10
	}
	r := make([]uint32, rlen)
	i := 0
	for k := range m {
		r[i] = k
		i++
		if i >= rlen {
			break
		}
	}
	return r
}

func shrinkString(s string) string {
	l := len(s)
	if l > 20 {
		return s[:10] + "..." + s[l-10:l]
	}
	return s
}
