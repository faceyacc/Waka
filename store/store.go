package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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

type Command struct {
	Action string
	Key    string
	Value  string
}

func (f *fsm) Apply(l *raft.Log) interface{} {
	log.Info("fsm.Apply called", "type", hclog.Fmt("%d", l.Type), "data", hclog.Fmt("%s", l.Data))

	var cmd Command

	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		log.Error("failed command unmarshal", "error", err)
		return nil
	}

	ctx := context.Background()
	switch cmd.Action {
	case "set":
		return f.localSet(ctx, cmd.Key, cmd.Value)
	case "delete":
		return f.localDelete(ctx, cmd.Key)
	default:
		log.Error("unknown command", "command", cmd, "log", l)
	}
	return nil
}

/* Returns a snapshot of the FS
 */
// Note: can can change to *fsmSnapshot
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	log.Info("fsm.Snapshot called")
	data, err := f.loadData(context.Background())
	if err != nil {
		return nil, err
	}

	encodedData, err := encode(data)
	if err != nil {
		return nil, err
	}
	return &fsmSnapshot{data: encodedData}, nil
}

/* Pulls data out of an io pipe and writes output to a disk
 */
func (f *fsm) Restore(old io.ReadCloser) error {
	log.Info("fs.Restore called")
	b, err := ioutil.ReadAll(old)
	if err != nil {
		return err
	}

	data, err := decode(b)
	if err != nil {
		return err
	}

	return f.saveData(context.Background(), data)
}

func (f *fsm) localSet(ctx context.Context, key, value string) error {
	data, err := f.loadData(ctx)
	if err != nil {
		return err
	}

	data[key] = value
	return f.saveData(ctx, data)
}

func (f *fsm) localGet(ctx context.Context, key string) (string, error) {
	data, err := f.loadData(ctx)
	if err != nil {
		return "", fmt.Errorf("load: %w", err)
	}
	return data[key], nil
}

func (f *fsm) localDelete(ctx context.Context, key string) error {
	data, err := f.loadData(ctx)
	if err != nil {
		return nil
	}
	delete(data, key)

	return f.saveData(ctx, data)
}

// Loads data using a Lock
func (f *fsm) loadData(ctx context.Context) (map[string]string, error) {
	empty := map[string]string{}
	if f.lock == nil {
		f.lock = flock.New(f.dataFile)
	}
	defer f.lock.Close()

	// // Set Lock
	locked, err := f.lock.TryLockContext(ctx, time.Millisecond)
	if err != nil {
		return empty, fmt.Errorf("trylock: %w", err)
	}

	if locked {
		// Check if the file exists and create it if it's missing
		if _, err := os.Stat(f.dataFile); os.IsNotExist(err) {
			emptyData, err := encode(map[string]string{})
			if err != nil {
				return empty, fmt.Errorf("encode: %w", err)
			}

			if err := ioutil.WriteFile(f.dataFile, emptyData, 0644); err != nil {
				return empty, fmt.Errorf("write: %w", err)
			}
		}
		content, err := ioutil.ReadFile(f.dataFile)
		if err != nil {
			return empty, fmt.Errorf("write: %w", err)
		}
		if err := f.lock.Unlock(); err != nil {
			return empty, fmt.Errorf("write: %w", err)
		}
		return decode(content)
	}
	return empty, fmt.Errorf("couldn't get lock")
}

// Saves data using a Lock
func (f *fsm) saveData(ctx context.Context, data map[string]string) error {
	encodedData, err := encode(data)
	if err != nil {
		return err
	}

	if f.lock == nil {
		f.lock = flock.New(f.dataFile)
	}
	defer f.lock.Close()

	// Set Lock
	locked, err := f.lock.TryLockContext(ctx, time.Millisecond)
	if err != nil {
		return err
	}

	if locked {
		if err := ioutil.WriteFile(f.dataFile, encodedData, 0644); err != nil {
			return err
		}
		if err := f.lock.Unlock(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("couldn't get lock")
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	log.Info("fsmSnapshot.Persist called")
	// Store snapshot state
	if _, err := sink.Write(s.data); err != nil {
		return err
	}
	defer sink.Close()

	return nil
}

func (s *fsmSnapshot) Release() {
	log.Info("fsmSnapsnot.Release called")
}

func encode(data map[string]string) ([]byte, error) {
	var encodedData map[string]string

	for k, v := range data {
		ek := base64.URLEncoding.EncodeToString([]byte(k))
		ev := base64.URLEncoding.EncodeToString([]byte(v))
		encodedData[ek] = ev
	}
	return json.Marshal(encodedData)
}

func decode(data []byte) (map[string]string, error) {
	var jsonData map[string]string

	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, err
	}

	returnData := map[string]string{}
	for k, v := range jsonData {
		dk, err := base64.URLEncoding.DecodeString(k)
		if err != nil {
			return nil, err
		}

		dv, err := base64.URLEncoding.DecodeString(v)
		if err != nil {
			return nil, err
		}
		returnData[string(dk)] = string(dv)
	}

	return returnData, nil
}

// Set sets a value for a key
func (cfg *Config) Set(ctx context.Context, key string, value string) error {
	// Check to see if caller is Leader
	if cfg.raft.State() != raft.Leader {
		return fmt.Errorf("not leader")
	}

	cmd, err := json.Marshal(Command{Action: "set", Key: key, Value: value})
	if err != nil {
		return fmt.Errorf("marshaling command %w", err)
	}
	l := cfg.raft.Apply(cmd, time.Minute)
	return l.Error()
}

// Delete removes a key and its value from the store.
func (cfg *Config) Delete(ctx context.Context, key string) error {
	if cfg.raft.State() != raft.Leader {
		return fmt.Errorf("not leader")
	}
	cmd, err := json.Marshal(Command{Action: "delete", Key: key})
	if err != nil {
		return fmt.Errorf("marhaling command %w", err)
	}
	l := cfg.raft.Apply(cmd, time.Minute)
	return l.Error()
}

// Get gets the value for a key.
func (cfg *Config) Get(ctx context.Context, key string) (string, error) {
	return cfg.fsm.localGet(ctx, key)
}

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

// Utility function to transform raft address to a url
func RaftAddressToHTTP(ldr raft.ServerAddress) *url.URL {
	url, err := url.Parse(string(ldr))
	if err != nil {
		fmt.Errorf("Address not found %w", err)
	}
	return url
}
