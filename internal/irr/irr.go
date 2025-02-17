package irr

import (
	"context"
	"fmt"
	"github.com/natesales/pathvector/internal/config"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// PrefixSet uses bgpq4 to generate a prefix filter and return only the filter lines
func PrefixSet(asSet string, family uint8, irrServer string, queryTimeout uint) ([]string, error) {
	// Run bgpq4 for BIRD format with aggregation enabled
	cmdArgs := fmt.Sprintf("-h %s -Ab%d %s", irrServer, family, asSet)
	log.Debugf("Running bgpq4 %s", cmdArgs)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(queryTimeout))
	defer cancel()
	cmd := exec.CommandContext(ctx, "bgpq4", strings.Split(cmdArgs, " ")...)
	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var prefixes []string
	for i, line := range strings.Split(string(stdout), "\n") {
		if i == 0 { // Skip first line, as it is the definition line
			continue
		}
		if strings.Contains(line, "];") { // Skip last line and return
			break
		}
		// Trim whitespace and remove the comma, then append to the prefixes slice
		prefixes = append(prefixes, strings.TrimSpace(strings.TrimRight(line, ",")))
	}

	return prefixes, nil
}

// Update updates a peer's IRR prefix set
func Update(peerData *config.Peer, irrServer string, queryTimeout uint) error {
	// Check for empty as-set
	if peerData.ASSet == nil || *peerData.ASSet == "" {
		return fmt.Errorf("peer has filter-irr enabled and no as-set defined")
	}

	// Does the peer have any IPv4 or IPv6 neighbors?
	var hasNeighbor4, hasNeighbor6 bool
	if peerData.NeighborIPs != nil {
		for _, n := range *peerData.NeighborIPs {
			if strings.Contains(n, ".") {
				hasNeighbor4 = true
			} else if strings.Contains(n, ":") {
				hasNeighbor6 = true
			}
		}
	}

	prefixesFromIRR4, err := PrefixSet(*peerData.ASSet, 4, irrServer, queryTimeout)
	if err != nil {
		return fmt.Errorf("unable to get IPv4 IRR prefix list from %s", *peerData.ASSet)
	}
	if peerData.PrefixSet4 == nil {
		peerData.PrefixSet4 = &[]string{}
	}
	pfx4 := append(*peerData.PrefixSet4, prefixesFromIRR4...)
	peerData.PrefixSet4 = &pfx4
	if len(pfx4) == 0 && hasNeighbor4 {
		return fmt.Errorf("peer has IPv4 session(s) but no IPv4 prefixes")
	}

	prefixesFromIRR6, err := PrefixSet(*peerData.ASSet, 6, irrServer, queryTimeout)
	if err != nil {
		return fmt.Errorf("unable to get IPv6 IRR prefix list from %s", *peerData.ASSet)
	}
	if peerData.PrefixSet6 == nil {
		peerData.PrefixSet6 = &[]string{}
	}
	pfx6 := append(*peerData.PrefixSet6, prefixesFromIRR6...)
	peerData.PrefixSet6 = &pfx6
	if len(pfx6) == 0 && hasNeighbor6 {
		return fmt.Errorf("peer has IPv6 session(s) but no IPv6 prefixes")
	}

	return nil // nil error
}
