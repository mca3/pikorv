package gateway

import (
	"context"
	"sync"
)

type sendReq struct {
	Device int64
	Msg    gatewayMsg
}

var wg sync.WaitGroup
var sendChan chan sendReq

// InitWorkers initializes the amount of gateway workers that are available to
// send messages.
func InitWorkers(num int, queue int) {
	sendChan = make(chan sendReq, queue)

	for i := 0; i < num; i++ {
		wg.Add(1)
		go worker()
	}
}

// JoinWorkers kills all workers that are supposed to be sending messages.
// Blocks execution until all workers have exhausted the queue.
func JoinWorkers() {
	close(sendChan)
	wg.Wait()
}

func worker() {
	defer wg.Done()

	for req := range sendChan {
		gc := findGatewayDevice(req.Device)
		if gc == nil {
			continue
		}

		gc.send(context.Background(), req.Msg)
	}
}
