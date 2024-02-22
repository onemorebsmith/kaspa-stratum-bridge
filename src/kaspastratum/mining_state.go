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
	Jobs        map[uint64]*appmessage.RPCBlock
	JobLock     sync.Mutex
	jobCounter  uint64
	bigDiff     big.Int
	initialized bool
	useBigJob   bool
	connectTime time.Time
	stratumDiff *kaspaDiff
	maxJobs     uint8
}

func MiningStateGenerator() any {
	return &MiningState{
		Jobs:        make(map[uint64]*appmessage.RPCBlock, maxjobs),
		JobLock:     sync.Mutex{},
		connectTime: time.Now(),
		maxJobs:     maxjobs,
	}
}

func GetMiningState(ctx *gostratum.StratumContext) *MiningState {
	return ctx.State.(*MiningState)
}

func (ms *MiningState) AddJob(job *appmessage.RPCBlock) uint64 {
	ms.JobLock.Lock()
	ms.jobCounter++
	idx := ms.jobCounter
	ms.Jobs[idx%maxjobs] = job
	ms.JobLock.Unlock()
	return idx
}

func (ms *MiningState) GetJob(id uint64) (*appmessage.RPCBlock, bool) {
	ms.JobLock.Lock()
	job, exists := ms.Jobs[id%maxjobs]
	ms.JobLock.Unlock()
	return job, exists
}
