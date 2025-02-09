package sharded

import (
	"log"
	"math/rand"
	"time"

	"golang.org/x/mod/semver"

	pbmt "github.com/percona/percona-backup-mongodb/pbm"
)

func (c *Cluster) IncrementalBackup(mongoVersion string) {
	inRange := lte
	// mongo v4.2 may not recover an oplog entry w/ timestamp equal
	// to the `BackLastWrite` (see https://jira.mongodb.org/browse/SERVER-54005).
	// Despite in general we expect to be restored all entries with `timestamp <= BackLastWrite`
	// in v4.2 we should expect only `timestamp < BackLastWrite`
	if semver.Compare(mongoVersion, "v4.2") == 0 {
		inRange = lt
	}

	rand.Seed(time.Now().UnixNano())
	counters := make(map[string]scounter)
	for name, shard := range c.shards {
		c.bcheckClear(name, shard)
		dt, cancel := c.bcheckWrite(name, shard, time.Millisecond*10*time.Duration(rand.Int63n(49)+1))
		counters[name] = scounter{
			data:   dt,
			cancel: cancel,
		}
	}

	bcpName := c.backup(pbmt.IncrementalBackup, "--base")
	c.BackupWaitDone(bcpName)
	time.Sleep(time.Second * 1)

	for i := 0; i < 3; i++ {
		bcpName = c.backup(pbmt.IncrementalBackup)
		c.BackupWaitDone(bcpName)
		time.Sleep(time.Second * 1)
	}

	sts, _ := c.pbm.RunCmd("pbm", "status", "-s", "backups")
	log.Println(sts)

	for _, c := range counters {
		c.cancel()
	}

	bcpMeta, err := c.mongopbm.GetBackupMeta(bcpName)
	if err != nil {
		log.Fatalf("ERROR: get backup '%s' metadata: %v\n", bcpName, err)
	}
	// fmt.Println("BCP_LWT:", bcpMeta.LastWriteTS)

	c.DeleteBallast()
	for name, shard := range c.shards {
		c.bcheckClear(name, shard)
	}

	c.PhysicalRestore(bcpName)

	for name, shard := range c.shards {
		c.bcheckCheck(name, shard, <-counters[name].data, bcpMeta.LastWriteTS, inRange)
	}
}
