package kaspastratum

import (
	"math/big"
	"sync"
	"time"

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
	connectTime time.Time
	stratumDiff *kaspaDiff
}

func MiningStateGenerator() any {
	return &MiningState{
		Jobs:        map[int]*appmessage.RPCBlock{},
		JobLock:     sync.Mutex{},
		connectTime: time.Now(),
	}
}

func GetMiningState(ctx *gostratum.StratumContext) *MiningState {
	return ctx.State.(*MiningState)
}

func (ms *MiningState) AddJob(job *appmessage.RPCBlock) int {
	ms.jobCounter++
	idx := ms.jobCounter
	ms.JobLock.Lock()
	ms.Jobs[idx%maxjobs] = job
	ms.JobLock.Unlock()
	return idx
}

func (ms *MiningState) GetJob(id int) (*appmessage.RPCBlock, bool) {
	ms.JobLock.Lock()
	job, exists := ms.Jobs[id%maxjobs]
	ms.JobLock.Unlock()
	return job, exists
}
