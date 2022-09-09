package kaspastratum

import (
	"math/big"
	"sync"

	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/onemorebsmith/kaspastratum/src/gostratum"
)

const maxjobs = 32

type MiningState struct {
	Jobs        map[int]*appmessage.RPCBlock
	JobLock     sync.Mutex
	jobCounter  int
	bigDiff     big.Int
	initialized bool
	useBigJob   bool
}

func MiningStateGenerator() any {
	return &MiningState{
		Jobs:    map[int]*appmessage.RPCBlock{},
		JobLock: sync.Mutex{},
	}
}

func GetMiningState(ctx *gostratum.StratumContext) *MiningState {
	return ctx.State.(*MiningState)
}

func (ms *MiningState) AddJob(job *appmessage.RPCBlock) int {
	ms.jobCounter++
	idx := ms.jobCounter % maxjobs
	ms.JobLock.Lock()
	ms.Jobs[idx] = job
	ms.JobLock.Unlock()
	return idx
}

func (ms *MiningState) GetJob(id int) *appmessage.RPCBlock {
	ms.JobLock.Lock()
	job := ms.Jobs[id]
	ms.JobLock.Unlock()
	return job
}
