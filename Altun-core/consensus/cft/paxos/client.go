package paxos

import (
	"strconv"

	"github.com/wtwinlab/Altun/layers/consensus/cft"
	"github.com/wtwinlab/Altun/layers/consensus/lib/log"
)

// Client overwrites read operation for Paxos
type Client struct {
	*cft.HTTPClient
	ballot cft.Ballot
}

func NewClient(id cft.ID) *Client {
	return &Client{
		HTTPClient: cft.NewHTTPClient(id),
	}
}

// Get implements cft.Client interface
// there are three reading modes:
// (1) read as normal command
// (2) read from leader with current ballot number
// (3) read from quorum of replicas with barrier
func (c *Client) Get(key cft.Key) (cft.Value, error) {
	c.HTTPClient.CID++
	switch *read {
	case "leader":
		return c.readLeader(key)
	case "quorum":
		return c.readQuorum(key)
	case "any":
		return c.readAny(key)
	default:
		return c.HTTPClient.Get(key)
	}
}

func (c *Client) Put(key cft.Key, value cft.Value) error {
	c.HTTPClient.CID++
	_, meta, err := c.RESTPut(c.ID, key, value)
	if err == nil {
		b := cft.NewBallotFromString(meta[HTTPHeaderBallot])
		if b > c.ballot {
			c.ballot = b
		}
	}

	return err
}

func (c *Client) readLeader(key cft.Key) (cft.Value, error) {
	if c.ballot == 0 {
		v, meta, err := c.HTTPClient.RESTGet(c.ID, key)
		c.ballot = cft.NewBallotFromString(meta[HTTPHeaderBallot])
		return v, err
	}
	// check ballot number
	v, meta, err := c.HTTPClient.RESTGet(c.ballot.ID(), key)
	b := cft.NewBallotFromString(meta[HTTPHeaderBallot])
	if b > c.ballot {
		c.ballot = b
	}
	return v, err
}

func (c *Client) readQuorum(key cft.Key) (cft.Value, error) {
	majority := c.N/2 + 1
	barrier := -1
	numReachedBarrier := 0
	numInProgress := 0
	var value cft.Value

	// quorum read
	values, metadatas := c.QuorumGet(key)
	for i, v := range values {
		slot, err := strconv.Atoi(metadatas[i][HTTPHeaderSlot])
		if err != nil {
			log.Error(err)
			continue
		}
		inProgress, err := strconv.ParseBool(metadatas[i][HTTPHeaderInProgress])
		if err != nil {
			log.Error(err)
			continue
		}
		if inProgress {
			numInProgress++
		}
		if slot > barrier {
			barrier = slot
			numReachedBarrier = 1
			value = v
		} else if slot == barrier {
			numReachedBarrier++
		}
	}

	// wait for slot to be executed by any node
	for numInProgress > 0 && numReachedBarrier < majority {
		// read from random node
		_, metadata, err := c.HTTPClient.RESTGet("", key)
		if err != nil {
			return nil, err
		}
		// get executed slot
		execute, err := strconv.Atoi(metadata[HTTPHeaderExecute])
		if err != nil {
			log.Error(err)
			continue
		}
		if execute >= barrier {
			break
		}

		// get highest accepted slot
		slot, err := strconv.Atoi(metadata[HTTPHeaderSlot])
		if err != nil {
			log.Error(err)
			continue
		}
		if slot >= barrier {
			numReachedBarrier++
		}
	}

	return value, nil
}

func (c *Client) readAny(key cft.Key) (cft.Value, error) {
	v, _, err := c.HTTPClient.RESTGet(c.ID, key)
	return v, err
}
