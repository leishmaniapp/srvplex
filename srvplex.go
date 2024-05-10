package srvplex

import (
	"context"
	"sync"

	"github.com/leishmaniapp/srvplex/internal/tools"
)

// SrvHdlr Defines the interface for compatible server handlers
type SrvHdlr interface {
	// Serve Start the server execution
	// wg: Once the server stops, the wg must be marked as done
	// stopch: Once the server is intenteded to stop a message will be recieved
	// errorch: Async error communication to the multiplexer
	Serve(wg *sync.WaitGroup, stopch <-chan struct{}, errch chan<- error)

	// ForceKill stops the server immediately
	ForceKill()
}

// Multiplexer runs multiple [SrvHdlr] at the same time
type Multiplexer struct {
	// Store the server alongside the stop channel
	servers []tools.Tuple[SrvHdlr, chan struct{}]
	// Error channel, recieves errors
	errorch chan error
	// Wait group
	wg *sync.WaitGroup
	// Register an error handler
	errhdlr func(error)
}

// NewServerMultiplex creates a server multiplexer
func NewServerMultiplex(errhdlr func(error)) *Multiplexer {
	return &Multiplexer{
		wg:      new(sync.WaitGroup),
		errhdlr: errhdlr,
		// Create the error channel for broadcasting errors
		errorch: make(chan error, 1),
	}
}

// AddServer add a [SrvHdlr] to the multiplexer
func (m *Multiplexer) AddServer(s SrvHdlr) {
	// Create the stop channel (make it buffered for non-blocking operation)
	stopchan := make(chan struct{}, 1)
	// Append the server with its channel
	m.servers = append(m.servers, tools.Tuple[SrvHdlr, chan struct{}]{
		First: s, Second: stopchan,
	})
}

// Run all the registered multiplexers (intended to be used within goroutine)
func (m *Multiplexer) Run() {
	// Listen for errors in separate goroutines
	go func() {
		for {
			// Read error from the servers
			err := <-m.errorch
			// Call the error handler
			m.errhdlr(err)
		}
	}()

	// Start all the servers
	for _, v := range m.servers {
		// Call servers async
		go v.First.Serve(m.wg, v.Second, m.errorch)
		// Add to the WorkGroup
		m.wg.Add(1)
	}
}

// Stop running all services
func (m *Multiplexer) Stop(ctx context.Context) error {
	// Send the stop signal to every server
	for _, v := range m.servers {
		select {
		// Send the stop signal
		case v.Second <- struct{}{}:
		// Context timeout, force quit
		case <-ctx.Done():
			// Force quit all the servers
			for _, v := range m.servers {
				v.First.ForceKill()
			}
			// Return the error
			return ctx.Err()
		}
	}

	// Stop the server with the context
	waitchan := make(chan struct{})

	go func() {
		// Wait for all servers to stop
		m.wg.Wait()
		// Send the signal
		waitchan <- struct{}{}
	}()

	// Either timout or server stop
	select {
	case <-ctx.Done():
		// Force quit all the servers
		for _, v := range m.servers {
			v.First.ForceKill()
		}
		// Return the error
		return ctx.Err()

	// Successfully stopped all servers
	case <-waitchan:
		return nil
	}
}
