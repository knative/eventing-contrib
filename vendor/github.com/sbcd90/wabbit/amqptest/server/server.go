package server

import (
	"errors"
	"fmt"
	"sync"

	"github.com/sbcd90/wabbit"
	"github.com/sbcd90/wabbit/utils"
)

const (
	MaxChannels int = 2 << 10
)

var (
	servers map[string]*AMQPServer
	mu      *sync.Mutex
)

func init() {
	servers = make(map[string]*AMQPServer)

	mu = &sync.Mutex{}
}

// AMQPServer is a fake AMQP server. It handle the fake TCP connection
type AMQPServer struct {
	running bool
	amqpuri string

	notifyChans map[string]*utils.ErrBroadcast
	channels    map[string][]*Channel
	vhost       *VHost
}

// NewServer returns a new fake amqp server
func newServer(amqpuri string) *AMQPServer {
	return &AMQPServer{
		amqpuri:     amqpuri,
		notifyChans: make(map[string]*utils.ErrBroadcast),
		channels:    make(map[string][]*Channel),
		vhost:       NewVHost("/"),
	}
}

// CreateChannel returns a new fresh channel
func (s *AMQPServer) CreateChannel(connID string, conn wabbit.Conn) (wabbit.Channel, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := s.channels[connID]; !ok {
		s.channels[connID] = make([]*Channel, 0, MaxChannels)
	}

	channels := s.channels[connID]

	if len(channels) >= MaxChannels {
		return nil, fmt.Errorf("Channel quota exceeded, Wabbit"+
			" supports only %d fake channels for tests.", MaxChannels)
	}

	ch := NewChannel(s.vhost)

	channels = append(channels, ch)
	s.channels[connID] = channels

	tempCh := make(chan wabbit.Error)
	conn.NotifyClose(tempCh)
	go func() {
		for err := range tempCh {
			ch.errSpread.Write(err)
		}
		close(tempCh)
	}()

	return ch, nil
}

// Start a new AMQP server fake-listening on host:port
func (s *AMQPServer) Start() error {
	mu.Lock()
	defer mu.Unlock()

	s.running = true
	return nil
}

// Stop the fake server
func (s *AMQPServer) Stop() error {
	mu.Lock()
	defer mu.Unlock()

	s.running = false

	for _, c := range s.notifyChans {
		c.Write(utils.NewError(
			utils.ChannelError,
			"channel/connection is not open",
			false,
			false,
		))
	}

	s.notifyChans = make(map[string]*utils.ErrBroadcast)

	for _, chanMap := range s.channels {
		for _, eachChan := range chanMap {
			eachChan.Close()
		}
	}

	return nil
}

// NewServer starts a new fake server
func NewServer(amqpuri string) *AMQPServer {
	var amqpServer *AMQPServer

	mu.Lock()
	defer mu.Unlock()

	amqpServer = servers[amqpuri]
	if amqpServer == nil {
		amqpServer = newServer(amqpuri)
		servers[amqpuri] = amqpServer
	}

	return amqpServer
}

func (s *AMQPServer) addNotify(connID string, nchan *utils.ErrBroadcast) {
	mu.Lock()
	defer mu.Unlock()
	s.notifyChans[connID] = nchan
}

func (s *AMQPServer) delNotify(connID string) {
	mu.Lock()
	defer mu.Unlock()
	delete(s.notifyChans, connID)
}

func getServer(amqpuri string) (*AMQPServer, error) {
	mu.Lock()
	defer mu.Unlock()

	amqpServer := servers[amqpuri]

	if amqpServer == nil || amqpServer.running == false {
		return nil, errors.New("Network unreachable")
	}

	return amqpServer, nil
}

func Connect(amqpuri string, connID string, errBroadcast *utils.ErrBroadcast) (*AMQPServer, error) {
	amqpServer, err := getServer(amqpuri)

	if err != nil {
		return nil, err
	}

	amqpServer.addNotify(connID, errBroadcast)
	return amqpServer, nil
}

func Close(amqpuri string, connID string) error {
	amqpServer, err := getServer(amqpuri)

	if err != nil {
		return errors.New("Failed to close connection")
	}

	amqpServer.delNotify(connID)
	return nil
}
