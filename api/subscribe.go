package api

import "sync"

//SubscribeService allows a client to subscribe to update messages
type SubscribeService struct {
	subscribers map[int]chan int
	lastID      int
	mu          *sync.Mutex
	control     chan int
}

func (s *SubscribeService) service() {
	for {
		id := <-s.control
		s.mu.Lock()
		for _, sub := range s.subscribers {
			sub <- id
		}
		s.mu.Unlock()
	}
}

//NewSubscribeService creates a new SubscribeService
func NewSubscribeService() *SubscribeService {
	s := &SubscribeService{
		subscribers: make(map[int]chan int),
		lastID:      0,
		mu:          new(sync.Mutex),
		control:     make(chan int),
	}
	go s.service()

	return s
}

//Subscribe subscribes a client and returns a chan to listen on
func (s *SubscribeService) Subscribe() (id int, c chan int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastID++

	c = make(chan int)
	s.subscribers[s.lastID] = c

	return s.lastID, c
}

//Unsubscribe unsubscribes the client with the given id from the service
func (s *SubscribeService) Unsubscribe(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.subscribers, id)
}

//Notify causes the service to notify all subscribers
func (s *SubscribeService) Notify(id int) {
	s.control <- id
}
