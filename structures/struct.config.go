package structures

import (
	"os"
	log "github.com/Sirupsen/logrus"
)

type Configuration struct {
	Local struct {
			HCInterval 	int `json:"hc_interval"`
			FOCInterval 	int `json:"foc_interval"`
			FOCLimit 	int `json:"foc_limit"`
			Role 		string `json:"role"`
			TLS 		bool `json:"tls"`
			Configured 	bool `json:"configured"`
	       } `json:"local"`
	Cluster Cluster `json:"cluster"`
}

type Cluster struct {
	Status string `json:"status"`
	Nodes Nodes `json:"nodes"`
}

type Nodes struct {
	Master ClusterDef `json:"master"`
	Slave ClusterDef `json:"slave"`
}

type ClusterDef struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

/**
 * Function used to validate that the values in the config
 * exist and are what we expect!
 */
func (c *Configuration) Validate() {
	var success bool = true

	// Make sure we have the "Local" Section
	if c.Local == (Configuration{}.Local) {
		log.Error("Invalid Config. Missing 'local' section.")
		success = false
	}

	// Make sure role is either master or slave
	if c.Local.Role != "master" && c.Local.Role != "slave" {
		log.Error("Invalid Config. Local role must either be 'master' or 'slave' under the General section.")
		success = false
	}

	// Make sure we have the "Cluster" Section
	if c.Cluster.Nodes == (Cluster{}.Nodes) {
		log.Error("Invalid Config. Missing 'nodes' section.")
		success = false
	}

	// Make sure we have the "Cluster" Section
	if c.Cluster == (Cluster{}) {
		log.Error("Invalid Config. Missing 'cluster' section.")
		success = false
	}

	if c.Cluster.Status == "" {
		log.Error("Invalid Config. Cluster 'status' is either not set or missing.")
		success = false
	}

	// Make sure we have the "Cluster->Master" Section
	if c.Cluster.Nodes.Master == (Cluster{}.Nodes.Master) {
		log.Error("Invalid Config. Missing 'master' in the cluster section.")
		success = false
	}

	// Make sure we have the "Cluster->Slave" Section
	if c.Cluster.Nodes.Slave == (Cluster{}.Nodes.Slave) {
		log.Error("Invalid Config. Missing 'slave' in the cluster section.")
		success = false
	}

	if success == false {
		// log why we exited?
		os.Exit(0)
	}
}
