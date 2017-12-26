package mapreduce

import "container/list"
import "fmt"

// WorkerInfo contains infomation about workers
type WorkerInfo struct {
	address string
	// You can add definitions here.
}

// KillWorkers clean up all workers by sending a Shutdown RPC to each one of them Collect
// the number of jobs each work has performed.
func (mr *MapReduce) KillWorkers() *list.List {
	l := list.New()
	for _, w := range mr.Workers {
		DPrintf("DoWork: shutdown %s\n", w.address)
		args := &ShutdownArgs{}
		var reply ShutdownReply
		ok := call(w.address, "Worker.Shutdown", args, &reply)
		if ok == false {
			fmt.Printf("DoWork: RPC %s shutdown error\n", w.address)
		} else {
			l.PushBack(reply.Njobs)
		}
	}
	return l
}

// RunWorker handle single worker jobs and state
func (mr *MapReduce) RunWorker(w string) {
L:
	for {
		select {
		case j := <-mr.mapJobs:
			args := &DoJobArgs{
				File:          mr.file,
				Operation:     Map,
				JobNumber:     j,
				NumOtherPhase: mr.nReduce,
			}
			var reply DoJobReply
			ok := call(w, "Worker.DoJob", args, &reply)
			if ok == false {
				fmt.Printf("DoWork: RPC %s shutdown error\n", w)
			}
			if reply.OK {
				mr.mapDone <- true
			} else {
				return
			}
		case <-mr.reduceStart:
			break L
		}
	}
	for {
		select {
		case j := <-mr.reduceJobs:
			args := &DoJobArgs{
				File:          mr.file,
				Operation:     Reduce,
				JobNumber:     j,
				NumOtherPhase: mr.nMap,
			}
			var reply DoJobReply
			ok := call(w, "Worker.DoJob", args, &reply)
			if ok == false {
				fmt.Printf("DoWork: RPC %s shutdown error\n", w)
			}
			if reply.OK {
				mr.reduceDone <- true
			} else {
				return
			}
		default:
			return
		}
	}
}

// RunMaster listen on register events and assign tasks
func (mr *MapReduce) RunMaster() *list.List {
	mapDone := 0
	reduceDone := 0

	for {
		select {
		case w := <-mr.registerChannel:
			go mr.RunWorker(w)
			mr.Workers[w] = &WorkerInfo{
				address: w,
			}
		case <-mr.mapDone:
			mapDone++
			if mapDone == mr.nMap {
				close(mr.reduceStart)
			}
		case <-mr.reduceDone:
			reduceDone++
			if reduceDone == mr.nReduce {
				return mr.KillWorkers()
			}
		}
	}
}
