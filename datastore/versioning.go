/*
	This file handles UUIDs and the version DAG.
*/

package datastore

import (
	"fmt"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/janelia-flyem/dvid/dvid"
)

// UUID is a 32 character hexidecimal string ("" if invalid) that uniquely identifies
// nodes in a datastore's DAG.  We need universally unique identifiers to prevent collisions
// during creation of child nodes by distributed DVIDs:
// http://en.wikipedia.org/wiki/Universally_unique_identifier
type UUID string

// NewUUID returns a UUID
func NewUUID() UUID {
	u := uuid.NewUUID()
	if u == nil || len(u) != 16 {
		return UUID("")
	}
	return UUID(fmt.Sprintf("%032x", []byte(u)))
}

// VersionId is a unique id that corresponds to one UUID in a datastore.  We limit the
// number of versions that can be present in a datastore to int16 and use that more compact 
// index compared to the full 16 byte UUID for construction of keys.
type VersionId int16

// VersionNode contains all information for a node in the version DAG like its parents,
// children, and provenance.
type VersionNode struct {
	Id         UUID
	Index      VersionId
	Locked     bool
	Parents    []UUID
	Children   []UUID
	Note       string
	Provenance string
	Created    time.Time
	Updated    time.Time
}

// VersionDAG is the directed acyclic graph of VersionNode and an index by UUID into
// the graph.
type VersionDAG struct {
	Head       UUID
	Nodes      []VersionNode
	VersionMap map[UUID]VersionId
}

// NewVersionDAG creates a version DAG and initializes the first unlocked node,
// assigning its UUID.
func NewVersionDAG() *VersionDAG {
	dag := VersionDAG{
		Head:  NewUUID(),
		Nodes: []VersionNode{},
	}
	t := time.Now()
	node := VersionNode{
		Id:      dag.Head,
		Index:   0,
		Created: t,
		Updated: t,
	}
	dag.Nodes = append(dag.Nodes, node)
	dag.VersionMap = map[UUID]VersionId{dag.Head: 0}
	return &dag
}

// LogInfo returns provenance information for all the version nodes.
func (dag *VersionDAG) LogInfo() string {
	text := "Versions:\n"
	for _, node := range dag.Nodes {
		text += fmt.Sprintf("%s  (%d)\n", node.Id, node.Index)
	}
	return text
}

// VersionIdFromString returns a UUID index given its string representation.  
// Partial matches are accepted as long as they are unique for a datastore.  So if
// a datastore has nodes with UUID strings 3FA22..., 7CD11..., and 836EE..., 
// we can still find a match even if given the minimum 3 letters.  (We don't
// allow UUID strings of less than 3 letters just to prevent mistakes.)
func (dag *VersionDAG) VersionIdFromString(str string) (id VersionId, err error) {
	var lastMatch VersionId
	numMatches := 0
	for uuid, id := range dag.VersionMap {
		dvid.Fmt(dvid.Debug, "Checking %s against %s\n", str, uuid)
		if strings.HasPrefix(string(uuid), str) {
			numMatches++
			lastMatch = id
		}
	}
	if numMatches > 1 {
		err = fmt.Errorf("More than one UUID matches %s!", str)
	} else if numMatches == 0 {
		err = fmt.Errorf("Could not find UUID with partial match to %s!", str)
	} else {
		id = lastMatch
	}
	return
}
