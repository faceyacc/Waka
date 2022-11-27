package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/pborman/uuid"

	"github.com/gofrs/flock"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

var (
	log = hclog.Default()
)

type fsm struct {
	dataFile string
	lock     *flock.Flock // Taken from "github.com/gofrs/flock"
}

type fsmSnapshot struct {
	data []byte
}

type Config struct {
	raft *raft.Raft // Taken from "github.com/hashicorp/raft"
	fsm  *fsm
}

func (c *Config) Set(ctx context.Context, key, value string) error {}

func (c *Config) Delete(ctx context.Context, key, value string) error {}

func (c *Config) Get(ctx context.Context, key, value string) (string, error) {}

// NewRaftSetup configures a raft server
func NewRaftSetup(storagePath, host, raftPort, raftLeader string) (*Config, error) {
	cfg := &Config{}

	if err := os.MkdirAll(storagePath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("setting up storage dir: %w", err)
	}

	cfg.fsm = &fsm{}
	cfg.fsm.dataFile = fmt.Sprintf("%s/data.json", storagePath)

	// Use raftbolt from "github.com/hashicorp/raft-boltdb"
	ss, err := raftboltdb.NewBoltStore(storagePath + "/stable")
	if err != nil {
		return nil, fmt.Errorf("building stable store: %w", err)
	}

	ls, err := raftboltdb.NewBoltStore(storagePath + "/log")
	if err != nil {
		return nil, fmt.Errorf("building log store: %w", err)
	}

	snaps, err := raft.NewFileSnapshotStoreWithLogger(storagePath+"/snaps", 5, log)
	if err != nil {
		return nil, fmt.Errorf("building snapshotstore: %w", err)
	}

	// Create a TCP transport
	fullTarget := fmt.Sprintf("%s:%s", host, raftPort)

	// Return an address of TCP end point.
	addr, err := net.ResolveTCPAddr("tcp", fullTarget)

	if err != nil {
		return nil, fmt.Errorf("getting address: %w", err)
	}
	trans, err := raft.NewTCPTransportWithLogger(fullTarget, addr, 10, 10*time.Second, log)
	if err != nil {
		return nil, fmt.Errorf("building transport: %w", err)
	}

	// Build Raft configuration
	raftSettings := raft.DefaultConfig()

	// Assign server a unique ID
	uuid := uuid.NewUUID()
	raftSettings.LocalID = raft.ServerID(uuid.URN())

	if err := raft.ValidateConfig(raftSettings); err != nil {
		return nil, fmt.Errorf("could not validate config: %w", err)
	}

	node, err := raft.NewRaft(raftSettings, cfg.fsm, ls, ss, snaps, trans)
	if err != nil {
		return nil, fmt.Errorf("could not create raft node: %w", err)
	}

	// Assing Raft feild to new Raft configuration from NewRaft
	cfg.raft = node

	// If Node Leader exists set raftLeader to that node
	if cfg.raft.Leader() != "" {
		raftLeader = string(cfg.raft.Leader())
	}

	// Make ourselves the leader
	if raftLeader == "" {
		raftConfig := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftSettings.LocalID,
					Address: raft.ServerAddress(fullTarget),
				},
			},
		}
		cfg.raft.BootstrapCluster(raftConfig)
	}

	// Watch the leader election forever
	leaderCh := cfg.raft.LeaderCh()
	go func() {
		for {
			select {
			// Send data to isLeader channel
			case isLeader := <-leaderCh:
				if isLeader {
					log.Info("cluster leadership acquired")

					// snapshot at random
					chance := rand.Int() % 10
					if chance == 0 {
						cfg.raft.Snapshot()
					}
				}
			}
		}
	}()

	// If not the leader, tell other nodes about new leader
	if raftLeader != "" {

		// Wait until leader might be ready
		time.Sleep(10 * time.Second)

		postJSON := fmt.Sprintf(`{"ID": %q, "Address": %q}`, &raftSettings.LocalID, fullTarget)
		resp, err := http.Post(
			raftLeader+"/raft/add",
			"application/json; charset=utf-8",
			strings.NewReader(postJSON))

		if err != nil {
			return nil, fmt.Errorf("failed adding self to leader %q: %w", raftLeader, err)
		}
		log.Debug("added self to leader", "leader", raftLeader, "response", resp)
	}
	return cfg, nil
}

// Function to Add requests to join a Raft cluster
// Returns an HTTP handler (funciton)
func (cfg *Config) AddHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		jw := json.NewEncoder(w)
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			jw.Encode(map[string]string{"error": err.Error()})
			return
		}
		log.Debug("got request", "body", string(body))

		var s *raft.Server
		if err := json.Unmarshal(body, &s); err != nil {
			log.Error("could not parse json", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			jw.Encode(map[string]string{"error": err.Error()})
			return
		}
		cfg.raft.AddVoter(s.ID, s.Address, 0, time.Minute)
		jw.Encode(map[string]string{"status": "success"})
	}
}

// Middleware passes the incoming request to the leader of the Raft cluster
func (cfg *Config) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.raft.State() != raft.Leader {
			ldr := cfg.raft.Leader()
			if ldr == "" {
				log.Error("leader address is empty")
				h.ServeHTTP(w, r)
				return
			}

			proxy := httputil.NewSingleHostReverseProxy(RaftAddressToHTTP(ldr))
			proxy.ServeHTTP(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}
