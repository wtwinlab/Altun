package orchestration

// Container mode for emulated wide-area deployments. With HS_DOCKER=1 every iago
// "host" is a container alias (e.g. altunwan-<site>-r1, resolved to its physical
// host by the ssh config), and:
//   - one worker per container, started with `docker exec -i <prefix><slot> env ... <exe> worker`;
//   - the exe is not uploaded: HS_DOCKER_EXE names a copy already on every host (bind-mounted);
//   - the test directory lives under HS_DOCKER_TMP (bind-mounted into the containers);
//   - HS_DOCKER_ASSIGN = "alias:r<id>,alias:c<id>,..." fixes which replica / client id runs where;
//   - a replica listens on the container's HS_REPLICA_PORT / HS_CLIENT_PORT (published 1:1).

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexhunt7/ssher"
	"github.com/relab/hotstuff"
	"github.com/relab/iago"
)

// DockerMode reports whether HS_DOCKER=1.
func DockerMode() bool { return os.Getenv("HS_DOCKER") == "1" }

// dockerContainer maps an alias "altunwan-<site>-r1" to its container name
// "<HS_DOCKER_CPREFIX>r1" (default prefix altunwan_).
func dockerContainer(alias string) string {
	i := strings.LastIndex(alias, "-")
	p := os.Getenv("HS_DOCKER_CPREFIX")
	if p == "" {
		p = "altunwan_"
	}
	return p + alias[i+1:]
}

// dockerIPs returns the container IP and host IP of alias from the file HS_DOCKER_IPS
// (lines "alias containerIP hostIP", written by the container deployment script).
func dockerIPs(alias string) []string {
	if !DockerMode() {
		return nil
	}
	b, err := os.ReadFile(os.Getenv("HS_DOCKER_IPS"))
	if err != nil {
		return nil
	}
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) >= 3 && f[0] == alias {
			return f[1:3]
		}
	}
	return nil
}

// dockerAssign parses HS_DOCKER_ASSIGN into alias -> (isReplica, id).
func dockerAssign() (map[string]hotstuff.ID, map[string]hotstuff.ID, error) {
	reps := map[string]hotstuff.ID{}
	clis := map[string]hotstuff.ID{}
	for _, kv := range strings.Split(os.Getenv("HS_DOCKER_ASSIGN"), ",") {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, ":", 2)
		if len(parts) != 2 || len(parts[1]) < 2 {
			return nil, nil, fmt.Errorf("bad HS_DOCKER_ASSIGN entry %q", kv)
		}
		id, err := strconv.Atoi(parts[1][1:])
		if err != nil {
			return nil, nil, fmt.Errorf("bad HS_DOCKER_ASSIGN entry %q", kv)
		}
		switch parts[1][0] {
		case 'r':
			reps[parts[0]] = hotstuff.ID(id)
		case 'c':
			clis[parts[0]] = hotstuff.ID(id)
		default:
			return nil, nil, fmt.Errorf("bad HS_DOCKER_ASSIGN entry %q", kv)
		}
	}
	return reps, clis, nil
}

// NewSSHGroupParallel is iago.NewSSHGroup with the ssh connections dialed concurrently
// (one connection per container alias; 40 sequential dials would add seconds per step).
func NewSSHGroupParallel(hostNames []string, sshConfigPath string) (iago.Group, error) {
	hosts := make([]iago.Host, len(hostNames))
	errs := make([]error, len(hostNames))
	var wg sync.WaitGroup
	for i, h := range hostNames {
		wg.Add(1)
		go func(i int, h string) {
			defer wg.Done()
			cfg, addr, err := ssher.ClientConfig(h, sshConfigPath)
			if err != nil {
				errs[i] = err
				return
			}
			// up to 8 aliases dial the same sshd at once (MaxStartups): retry a failed dial
			for try := 0; try < 4; try++ {
				if try > 0 {
					time.Sleep(time.Duration(200*try) * time.Millisecond)
				}
				if hosts[i], errs[i] = iago.DialSSH(h, addr, cfg); errs[i] == nil {
					return
				}
			}
		}(i, h)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return iago.Group{}, fmt.Errorf("%s: %w", hostNames[i], err)
		}
	}
	return iago.NewGroup(hosts), nil
}
