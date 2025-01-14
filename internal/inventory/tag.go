package inventory

import (
	"regexp"
	"strings"
)

const (
	// Regular expresion for extracting the Cluster's Name configured by `openshift-installer` from AWS Tags
	clusterNameRegexp = "kubernetes.io/cluster/(.*?)-.{5}$"
	// Regular expresion for extracting the InfrastructureID configured by `openshift-installer` from AWS Tags
	infraIDRegexp = "kubernetes.io/cluster/.*-(.{5}?)$"
	// Regular expresion for extracting the InfrastructureID configured by `openshift-installer` from AWS Tags
	clusterIDRegexp = "kubernetes.io/cluster/(.*)$"

	unknownClusterNameCode = "UNKNOWN-CLUSTER"
	unknownClusterIDCode   = "UNKNOWN-CLUSTER"
)

// Tag model generic tags as a Key-Value object
type Tag struct {
	// Tag's key
	Key string `db:"key" json:"key"`

	// Tag's Value
	Value string `db:"value" json:"value"`

	// InstanceName reference
	InstanceID string `db:"instance_id" json:"instance_id"`
}

// NewTag returns a new generic tag struct
func NewTag(key string, value string, instanceID string) *Tag {
	return &Tag{Key: key, Value: value, InstanceID: instanceID}
}

// lookForTagByKey looks for a Tag based on its Key and returns a pointer to it
func LookForTagByKey(key string, tags []Tag) *Tag {
	var resultTag Tag
	for _, tag := range tags {
		if tag.Key == key {
			return &resultTag
		}
	}
	return nil
}

// parseClusterName parses a Tag key to obtain the clusterName
func parseClusterName(key string) string {
	re := regexp.MustCompile(clusterNameRegexp)
	res := re.FindAllStringSubmatch(key, 1)

	// if there are no results, return empty string, if there are, return first match
	if len(res) <= 0 {
		return unknownClusterNameCode
	}
	return res[0][1]
}

// parseClusterName parses a Tag key to obtain the clusterName
func parseClusterID(key string) string {
	re := regexp.MustCompile(clusterNameRegexp)
	res := re.FindAllStringSubmatch(key, 1)

	// if there are no results, return empty string, if there are, return first match
	if len(res) <= 0 {
		return unknownClusterIDCode
	}
	return res[0][1]
}

// parseInfraID parses a Tag key to obtain the InfraID
func parseInfraID(key string) string {
	re := regexp.MustCompile(infraIDRegexp)
	res := re.FindAllStringSubmatch(key, 1)

	// if there are no results, return empty string, if there are, return first match
	if len(res) <= 0 {
		return ""
	}
	return res[0][1]
}

// GetOwnerFromTags looks for a tag with the key "Owner" and returns its value
func GetOwnerFromTags(tags []Tag) string {
	result := (LookForTagByKey("Owner", tags))
	if result != nil {
		return result.Key
	}
	return ""
}

func GetInstanceNameFromTags(tags []Tag) string {
	result := (LookForTagByKey("Name", tags))
	if result != nil {
		return result.Key
	}
	return ""
}

func GetClusterIDFromTags(tags []Tag) string {
	for _, tag := range tags {
		if strings.Contains(tag.Key, ClusterTagKey) {
			return parseClusterID(tag.Key)
		}
	}
	return unknownClusterNameCode
}

func GetClusterNameFromTags(tags []Tag) string {
	for _, tag := range tags {
		if strings.Contains(tag.Key, ClusterTagKey) {
			return parseClusterName(tag.Key)
		}
	}
	return unknownClusterNameCode
}

func GetInfraIDFromTags(tags []Tag) string {
	for _, tag := range tags {
		if strings.Contains(tag.Key, ClusterTagKey) {
			return parseInfraID(tag.Key)
		}
	}
	return unknownClusterNameCode
}
