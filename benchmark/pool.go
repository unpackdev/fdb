package benchmark

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ClientPool is a pool of clients that sends messages concurrently.
type ClientPool struct {
	totalClients      int
	messagesPerClient int
	poolWg            sync.WaitGroup
	latencyMutex      sync.Mutex
	latencyData       []time.Duration
	report            *Report
}

// NewClientPool creates a new client pool with the specified number of clients and messages per client.
func NewClientPool(totalClients, messagesPerClient int, report *Report) *ClientPool {
	return &ClientPool{
		totalClients:      totalClients,
		messagesPerClient: messagesPerClient,
		latencyData:       make([]time.Duration, 0, totalClients*messagesPerClient),
		report:            report,
	}
}

// Start sends messages concurrently using the pool of clients.
func (p *ClientPool) Start(ctx context.Context, suite TransportSuite) error {
	p.poolWg.Add(p.totalClients)
	startTime := time.Now()

	for i := 0; i < p.totalClients; i++ {
		go p.runClient(ctx, suite)
	}

	p.poolWg.Wait()

	p.report.TotalDuration = time.Since(startTime)
	return nil
}

// runClient simulates a client sending multiple messages to the server.
func (p *ClientPool) runClient(ctx context.Context, suite TransportSuite) {
	defer p.poolWg.Done()

	for i := 0; i < p.messagesPerClient; i++ {
		messageStartTime := time.Now()

		err := suite.Run(ctx)
		latency := time.Since(messageStartTime)

		p.latencyMutex.Lock()
		p.latencyData = append(p.latencyData, latency)
		p.latencyMutex.Unlock()

		if err != nil {
			fmt.Printf("Message %d failed: %v\n", i+1, err)
			p.report.FailedMessages++
		} else {
			p.report.SuccessMessages++
		}
	}
}

// Finalize aggregates the latency data and updates the benchmark report.
func (p *ClientPool) Finalize() {
	totalMessages := p.totalClients * p.messagesPerClient
	p.report.TotalMessages = totalMessages
	p.report.Throughput = float64(p.report.SuccessMessages) / p.report.TotalDuration.Seconds()

	// Calculate average latency
	var totalLatency time.Duration
	for _, latency := range p.latencyData {
		totalLatency += latency
	}
	if len(p.latencyData) > 0 {
		p.report.AvgLatency = totalLatency / time.Duration(len(p.latencyData))
	}

	// Update latency histogram
	p.report.LatencyHistogram = p.latencyData
}
