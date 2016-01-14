package clientpool

import (
	"errors"
	"math/rand"
	"strings"
	"sync"

	"github.com/cloudfoundry/gosteno"
)

var ErrorEmptyClientPool = errors.New("loggregator client pool is empty")

type ClientFactory interface {
	NewClient(url string) (Client, error)
}

type DopplerPool struct {
	logger        *gosteno.Logger
	clientFactory ClientFactory

	sync.RWMutex
	clients    map[string]Client
	clientList []Client

	nonLegacyServers map[string]string
	legacyServers    map[string]string
}

func NewDopplerPool(logger *gosteno.Logger, clientFactory ClientFactory) *DopplerPool {
	return &DopplerPool{
		logger:        logger,
		clientFactory: clientFactory,
	}
}

func (pool *DopplerPool) Set(all map[string]string, preferred map[string]string) {
	pool.Lock()

	if len(preferred) > 0 {
		pool.nonLegacyServers = preferred
	} else if len(all) > 0 {
		pool.nonLegacyServers = all
	} else {
		pool.nonLegacyServers = nil
	}

	pool.merge()
	pool.Unlock()
}

func (pool *DopplerPool) SetLegacy(all map[string]string, preferred map[string]string) {
	pool.Lock()

	if len(preferred) > 0 {
		pool.legacyServers = preferred
	} else if len(all) > 0 {
		pool.legacyServers = all
	} else {
		pool.legacyServers = nil
	}

	pool.merge()
	pool.Unlock()
}

func (pool *DopplerPool) Clients() []Client {
	defer pool.RUnlock()
	pool.RLock()
	clientList := make([]Client, len(pool.clientList))
	copy(clientList, pool.clientList)
	return clientList
}

func (pool *DopplerPool) RandomClient() (Client, error) {
	list := pool.Clients()

	if len(list) == 0 {
		return nil, ErrorEmptyClientPool
	}

	return list[rand.Intn(len(list))], nil
}

func (pool *DopplerPool) getClient(key, url string) Client {
	var client Client
	if client = pool.clients[key]; client == nil {
		return nil
	}

	// scheme://address
	if !((len(url) == len(client.Scheme())+3+len(client.Address())) &&
		strings.HasPrefix(url, client.Scheme()) && strings.HasSuffix(url, client.Address())) {
		return nil
	}

	return client
}

func (pool *DopplerPool) merge() {
	newClients := map[string]Client{}

	for key, u := range pool.nonLegacyServers {
		client := pool.getClient(key, u)
		if client == nil {
			var err error
			client, err = pool.clientFactory.NewClient(u)
			if err != nil {
				pool.logger.Errord(map[string]interface{}{
					"doppler": key, "url": u, "error": err,
				}, "Invalid url")
				continue
			}
		}
		newClients[key] = client
	}

	for key, u := range pool.legacyServers {
		if _, ok := newClients[key]; !ok {
			client := pool.getClient(key, u)
			if client == nil {
				var err error
				client, err = pool.clientFactory.NewClient(u)
				if err != nil {
					pool.logger.Errord(map[string]interface{}{
						"doppler": key, "url": u, "error": err,
					}, "Invalid url")
					continue
				}
			}
			newClients[key] = client
		}
	}

	for address, client := range pool.clients {
		if _, ok := newClients[address]; !ok {
			client.Close()
		}
	}

	newList := make([]Client, 0, len(newClients))
	for _, client := range newClients {
		newList = append(newList, client)
	}

	pool.clients = newClients
	pool.clientList = newList
}
