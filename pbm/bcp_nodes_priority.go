package pbm

import (
	"sort"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const defaultScore = 1.0

// NodesPriority groups nodes by priority according to
// provided scores. Basically nodes are grouped and sorted by
// descending order by score
type NodesPriority struct {
	m map[string]nodeScores
}

func NewNodesPriority() *NodesPriority {
	return &NodesPriority{make(map[string]nodeScores)}
}

// Add node with its score
func (n *NodesPriority) Add(rs, node string, sc float64) {
	s, ok := n.m[rs]
	if !ok {
		s = nodeScores{m: make(map[float64][]string)}
	}
	s.add(node, sc)
	n.m[rs] = s
}

// RS returns nodes `group and sort desc by score` for given replset
func (n *NodesPriority) RS(rs string) [][]string {
	return n.m[rs].list()
}

type agentScore func(AgentStat) float64

// BcpNodesPriority returns list nodes grouped by backup preferences
// in descended order. First are nodes with the highest priority.
// Custom coefficients might be passed. These will be ignored though
// if the config is set.
func (p *PBM) BcpNodesPriority(c map[string]float64, agents []AgentStat) (*NodesPriority, error) {
	cfg, err := p.GetConfig()
	if err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	// if cfg.Backup.Priority doesn't set apply defaults
	f := func(a AgentStat) float64 {
		if coeff, ok := c[a.Node]; ok && c != nil {
			return defaultScore * coeff
		} else if a.State == NodeStatePrimary {
			return defaultScore / 2
		} else if a.Hidden {
			return defaultScore * 2
		}
		return defaultScore
	}

	if cfg.Backup.Priority != nil || len(cfg.Backup.Priority) > 0 {
		f = func(a AgentStat) float64 {
			sc, ok := cfg.Backup.Priority[a.Node]
			if !ok || sc < 0 {
				return defaultScore
			}

			return sc
		}
	}

	return bcpNodesPriority(agents, f), nil
}

func bcpNodesPriority(agents []AgentStat, f agentScore) *NodesPriority {
	scores := NewNodesPriority()

	for _, a := range agents {
		if ok, _ := a.OK(); !ok {
			continue
		}

		scores.Add(a.RS, a.Node, f(a))
	}

	return scores
}

type nodeScores struct {
	idx []float64
	m   map[float64][]string
}

func (s *nodeScores) add(node string, sc float64) {
	nodes, ok := s.m[sc]
	if !ok {
		s.idx = append(s.idx, sc)
	}
	s.m[sc] = append(nodes, node)
}

func (s nodeScores) list() [][]string {
	ret := make([][]string, len(s.idx))
	sort.Sort(sort.Reverse(sort.Float64Slice(s.idx)))

	for i := range ret {
		ret[i] = s.m[s.idx[i]]
	}

	return ret
}

func (p *PBM) SetRSNomination(bcpName, rs string) error {
	n := BackupRsNomination{RS: rs, Nodes: []string{}}
	_, err := p.Conn.Database(DB).Collection(BcpCollection).
		UpdateOne(
			p.ctx,
			bson.D{{"name", bcpName}},
			bson.D{{"$addToSet", bson.M{"n": n}}},
		)

	return errors.WithMessage(err, "query")
}

func (p *PBM) GetRSNominees(bcpName, rsName string) (*BackupRsNomination, error) {
	bcp, err := p.GetBackupMeta(bcpName)
	if err != nil {
		return nil, err
	}

	for _, n := range bcp.Nomination {
		if n.RS == rsName {
			return &n, nil
		}
	}

	return nil, ErrNotFound
}

func (p *PBM) SetRSNominees(bcpName, rsName string, nodes []string) error {
	_, err := p.Conn.Database(DB).Collection(BcpCollection).UpdateOne(
		p.ctx,
		bson.D{{"name", bcpName}, {"n.rs", rsName}},
		bson.D{
			{"$set", bson.M{"n.$.n": nodes}},
		},
	)

	return err
}

func (p *PBM) SetRSNomineeACK(bcpName, rsName, node string) error {
	_, err := p.Conn.Database(DB).Collection(BcpCollection).UpdateOne(
		p.ctx,
		bson.D{{"name", bcpName}, {"n.rs", rsName}},
		bson.D{
			{"$set", bson.M{"n.$.ack": node}},
		},
	)

	return err
}
