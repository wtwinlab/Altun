/*
Copyright IBM Corp. 2018 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

         http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"sync/atomic"

	"github.com/IBM/mirbft/config"
	pb "github.com/IBM/mirbft/protos"
	"github.com/IBM/mirbft/tracing"
	"github.com/op/go-logging"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var log = logging.MustGetLogger("client")
var format = logging.MustStringFormatter(
	`%{time:2006/01/02 15:04:05.000000} %{shortfunc} %{message}`,
)

func main() {
	// Client setup
	configFile := arg(1)
	config.LoadFile(configFile)

	// Get output file prefix from command line.
	// Log and trace files will all begin with this
	outFilePrefix := os.Args[2]

	// Open status file
	status_file, err := os.Create("STATUS.sh")
	fatal(err)
	defer status_file.Close()

	status_file.WriteString("status=UP\n")

	logBackend := logging.NewLogBackend(os.Stdout, "", 0)
	backendFormatter := logging.NewBackendFormatter(logBackend, format)
	logging.SetBackend(backendFormatter)

	if config.Config.Logging == "debug" {
		logging.SetLevel(logging.DEBUG, "client")
	}
	if config.Config.Logging == "info" {
		logging.SetLevel(logging.INFO, "client")
	}
	if config.Config.Logging == "error" {
		logging.SetLevel(logging.ERROR, "client")
	}
	if config.Config.Logging == "critical" {
		logging.SetLevel(logging.CRITICAL, "client")
	}

	osns := make([]pb.ConsensusClient, 0)
	grpcConns := make([]*grpc.ClientConn, 0)

	id := config.Config.Id
	N := config.Config.N
	F := config.Config.F

	payloadSize := config.Config.RequestSize
	signed := config.Config.SignatureVerification
	useTLS := config.Config.UseTLS

	total := config.Config.RequestsPerClient
	sendParallelism := config.Config.Parallelism
	timeout := int64(0)
	if config.Config.RequestRate > 0 {
		timeout = int64(1000000000 / config.Config.RequestRate)
	}
	clients := config.Config.Clients

	leaderRotationDist := config.Config.BucketRotationPeriod
	batchSize := config.Config.BatchSizeBytes
	buckets := config.Config.Buckets

	receivers := config.Config.Receivers
	broadcast := config.Config.Broadcast
	dst := config.Config.Destination

	caCertFile := config.Config.Servers.CACertFile
	requestsPerBatch := batchSize / payloadSize
	period := requestsPerBatch * leaderRotationDist
	payload := make([]byte, payloadSize, payloadSize)

	// New consensus client
	c, err := New(id, N, F, receivers, buckets, period, dst, int64(total),
		osns, caCertFile, useTLS)

	c.inflight = make(chan struct{}, sendParallelism)
	c.stopCh = make(chan struct{})

	c.trace.Start(fmt.Sprintf("%s-%03d.trc", outFilePrefix, c.id), int32(c.id))
	defer c.trace.Stop()

	log.Infof("Preparing requests...")

	for j := 0; j < total; j++ {
		seq := uint64(j)
		c.queue[j] = c.makeRequest(payload, seq, signed)
	}

	if config.Config.Blocking {
		log.Infof("Requests added to queue. Bring up the servers and press enter...")
		fmt.Scanln()
	}

	for i, addr := range config.Config.Servers.Addresses {
		conn, err := c.AddOSN(addr)
		grpcConns = append(grpcConns, conn)
		defer grpcConns[i].Close()
		fatal(err)
	}

	waitc := make([]chan struct{}, c.n)
	stop := make(chan bool, 1) // buffered: the window timer must never block

	for inc := c.dst; inc < c.n+dst; inc++ {
		i := inc % c.n
		log.Infof("Opening stream to server %d", i)
		c.stream[i], err = c.servers[i].Request(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		waitc[i] = make(chan struct{})
		time.Sleep(500 * time.Millisecond)

		go func(i int) {
			for {
				resp, err := c.stream[i].Recv()
				if err == io.EOF {
					// read done.
					close(waitc[i])
					return
				}
				if err != nil {
					// Tolerate a failed replica (up to f): the original
					// log.Fatalf killed the whole client when any one replica
					// died, so no window result was written at all.
					log.Errorf("Failed to receive from replica %d: %v", i, err)
					close(waitc[i])
					return
				}
				if fmt.Sprintf("%s", resp.Response) == "DELIVERED" {
					// Request delivered
					log.Debugf("DELIVERED %d from %d", resp.Request.Seq, resp.Src)
					// Check if registered to all severs
					c.registrationsLock.Lock()
					if ok, _ := c.registrations[resp.Src]; !ok {
						c.registrations[resp.Src] = true
						if len(c.registrations) == N {
							c.registered <- true
						}
					}
					c.registrationsLock.Unlock()

					// Use the value returned by AddInt32: the original separate
					// LoadInt32(...) == F+1 check below raced between the
					// per-server receive goroutines (two goroutines could both
					// load F+2), so ~1% of requests were never counted as finished.
					nDelivered := atomic.AddInt32(&c.delivered[resp.Request.Seq], 1)
					c.sentTimestampsLock.RLock()
					c.trace.Event(tracing.RESP_RECEIVE, int64(resp.Request.Seq), time.Now().UnixNano()/1000-c.sentTimestamps[uint64(id)])
					log.Infof("RESPONSE %d", resp.Request.Seq)
					c.sentTimestampsLock.RUnlock()
					if nDelivered == int32(F+1) {
						c.sentTimestampsLock.RLock()
						c.trace.Event(tracing.ENOUGH_RESP, int64(resp.Request.Seq), time.Now().UnixNano()/1000-c.sentTimestamps[uint64(id)])
						c.sentTimestampsLock.RUnlock()
						c.submitTimestampsLock.RLock()
						c.trace.Event(tracing.REQ_FINISHED, int64(resp.Request.Seq), time.Now().UnixNano()/1000-c.submitTimestamps[uint64(id)])
						c.submitTimestampsLock.RUnlock()
						log.Infof("FINISHED %d", resp.Request.Seq)
						c.recordCompletion(resp.Request.Seq)
						// closed loop: free one outstanding-request slot
						select {
						case <-c.inflight:
						default:
						}
						c.checkInOrderDelivery(int64(atomic.LoadInt64(&c.lastDelivered) + 1))
						if atomic.LoadInt64(&c.lastDelivered) == c.numRequests-1 {
							log.Infof("ALL DELIVERED")
						}
					}
				} else if fmt.Sprintf("%s", resp.Response) == "ACK" {
					// Request added to pending on server side
					log.Infof("ACK")
				} else if fmt.Sprintf("%s", resp.Response) == "BUCKETS" {

				} else {
					// Request could not be processed, put request back in the queue
					log.Infof("client: received %s from %d", fmt.Sprintf("%s", resp.Response), resp.Src)
				}
			}
		}(i)
	}

	log.Infof("REGISTERING")
	c.register()
	c.totalSent = int32(clients)

	<-c.registered

	log.Infof("START")
	atomic.StoreInt64(&c.windowStartUs, time.Now().UnixNano()/1000)

	// Make client stop after a predefined time, if configured. The timer
	// closes the measurement window and stops request submission.
	if config.Config.ClientRunTime != 0 {
		log.Infof("Setting up client timeout.")
		time.AfterFunc(time.Duration(config.Config.ClientRunTime)*time.Millisecond, func() {
			log.Infof("Stopping client on timeout.")
			atomic.StoreInt32(&c.stop, 1)
			close(c.stopCh)
			stop <- true
		})
	}

	//var wg sync.WaitGroup
	//// Submit requests in parallel
	//for i := 0; i < sendParallelism; i++ {
	//	wg.Add(1)
	//	go c.requestWorker(&wg, timeout, broadcast, requestsPerClient, clients, sendParallelism, i)
	//}
	//wg.Wait()

	c.submitRequests(timeout, broadcast, total, clients, sendParallelism)

	exhausted := atomic.LoadInt32(&c.stop) == 0
	if exhausted {
		log.Errorf("REQUESTS EXHAUSTED before the time window closed (%d requests)", total)
	}
	log.Infof("STOPPED SENDING")

	<-stop

	// Window closed: report the requests completed inside it, flush the
	// trace, then signal FINISHED so that the deployment scripts can collect
	// the trace and window.out files.
	endUs := time.Now().UnixNano() / 1000
	startUs := atomic.LoadInt64(&c.windowStartUs)
	done := atomic.LoadInt64(&c.windowDone)
	latSum := atomic.LoadInt64(&c.windowLatSumUs)
	persec := ""
	for i := range c.perSec {
		if i > 0 {
			persec += ","
		}
		persec += fmt.Sprintf("%d", atomic.LoadInt64(&c.perSec[i]))
	}
	line := fmt.Sprintf("WINDOW client=%d completed=%d elapsed_us=%d latsum_us=%d sent=%d exhausted=%v start_us=%d persec=%s\n",
		id, done, endUs-startUs, latSum, atomic.LoadInt64(&c.windowSent), exhausted, startUs, persec)
	if wf, err := os.Create("window.out"); err == nil {
		wf.WriteString(line)
		wf.Close()
	}
	log.Errorf("%s", line)
	c.trace.Stop()

	log.Infof("FINISHED %d", atomic.LoadInt64(&c.lastDelivered))
	status_file.WriteString("status=FINISHED\n")
	status_file.Sync()

	closed := make(chan struct{})
	go func() {
		for i := 0; i < N; i++ {
			c.stream[i].CloseSend()
			<-waitc[i]
		}
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
	}
	os.Exit(0)
}

// recordCompletion counts a request that reached F+1 DELIVERED responses
// while the measurement window is open, with its submit-to-commit latency.
func (c *Client) recordCompletion(seq uint64) {
	if atomic.LoadInt32(&c.stop) != 0 {
		return
	}
	c.submitTimestampsLock.RLock()
	ts, ok := c.submitTimestamps[seq]
	c.submitTimestampsLock.RUnlock()
	if !ok || ts < atomic.LoadInt64(&c.windowStartUs) {
		return
	}
	atomic.AddInt64(&c.windowDone, 1)
	if sec := (time.Now().UnixNano()/1000 - atomic.LoadInt64(&c.windowStartUs)) / 1000000; sec >= 0 && sec < int64(len(c.perSec)) {
		atomic.AddInt64(&c.perSec[sec], 1)
	}
	atomic.AddInt64(&c.windowLatSumUs, time.Now().UnixNano()/1000-ts)
}

// submitRequests runs a closed loop: at most `parallelism` requests are
// outstanding (a slot is freed when a request reaches F+1 DELIVERED), and the
// send rate is capped at config requestRate req/s per client (if > 0), like the
// cft benchmark's concurrency/throttle. It returns when the time window closes.
// (The original loop sent bursts of `parallelism` requests every 1/requestRate s
// with no bound on outstanding requests, i.e. an open-loop flood.)
func (c *Client) submitRequests(timeout int64, broadcast bool, numRequests, clients, parallelism int) {
	t0 := time.Now()
	sent := int64(0)
	// seq 0 was already sent and delivered as the registration request
	for seq := 1; seq < numRequests; seq++ {
		select {
		case c.inflight <- struct{}{}:
		case <-c.stopCh:
			log.Infof("Request submitter stopped by time window.")
			return
		}
		if timeout > 0 {
			if d := t0.Add(time.Duration(sent*timeout) * time.Nanosecond).Sub(time.Now()); d > 0 {
				time.Sleep(d)
			}
		}
		if atomic.LoadInt32(&c.stop) != 0 {
			log.Infof("Request submitter stopped by time window.")
			return
		}
		request := c.queue[seq]
		c.submitTimestampsLock.Lock()
		c.submitTimestamps[request.request.Seq] = time.Now().UnixNano() / 1000 // In us
		c.submitTimestampsLock.Unlock()
		if broadcast {
			c.broadcastRequest(request)
		} else {
			c.sendRequest(request)
		}
		sent++
		atomic.AddInt64(&c.windowSent, 1)
		atomic.AddInt32(&c.totalSent, int32(clients))
		if atomic.LoadInt32(&c.totalSent)%int32(c.period) == 0 {
			log.Infof("Rotating buckets")
			c.rotateBuckets()
		}
	}
	log.Infof("Request submitter DONE.")
	return
}

func fatal(err error) {
	if err != nil {
		log.Error(err)
	}
}

func arg(i int) string {
	if len(os.Args) < i {
		panic(errors.New("Insufficient arguments"))
	}
	input := os.Args[i]
	return input
}
