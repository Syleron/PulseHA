/*
   PulseHA - HA Cluster Daemon
   Copyright (C) 2017-2019  Andrew Zak <andrew@linux.com>

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as published
   by the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/
package config

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/syleron/pulseha/src/jsonHelper"
	"github.com/syleron/pulseha/src/utils"
	"io/ioutil"
	"os"
	"sync"
)

var (
	CONFIG_LOCATION = "/etc/pulseha/config.json"
)

type Config struct {
	Pulse  Local               `json:"pulseha"`
	Groups map[string][]string `json:"floating_ip_groups"`
	Nodes  map[string]Node     `json:"nodes"`
	sync.Mutex
}

type Local struct {
	HealthCheckInterval int    `json:"hcs_interval"`
	FailOverInterval    int    `json:"fos_interval"`
	FailOverLimit       int    `json:"fo_limit"`
	LocalNode           string `json:"local_node"`
	ClusterToken        string `json:"cluster_token"`
	LoggingLevel        string `json:"logging_level"`
}

type Nodes struct {
	Nodes map[string]Node
}

type Node struct {
	Hostname string              `json:"hostname"`
	IP       string              `json:"bind_address"`
	Port     string              `json:"bind_port"`
	IPGroups map[string][]string `json:"group_assignments"`
}

/**
Instantiate, setup and return our Config
*/
func New() *Config {
	cfg := Config{}
	if err := cfg.Load(); err != nil {
		log.Fatal(err)
	}
	return &cfg
}

// GetConfig - Returns a copy of the config
func (c *Config) GetConfig() Config {
	return *c
}

// NodeCount - Returns the total number of nodes in the configured cluster
func (c *Config) NodeCount() int {
	return len(c.Nodes)
}

// GetLocalNode - Return the local node hostname
func (c *Config) GetLocalNodeUUID() string {
	return c.Pulse.LocalNode
}

//
func (c *Config) GetLocalNode() Node {
	if node, ok := c.Nodes[c.Pulse.LocalNode]; ok {
		return node
	}
	panic("Local node does not exist in local config.")
}

// Load - Used to load the config into memory
func (c *Config) Load() error {
	c.Lock()
	defer c.Unlock()
	// Check to see if we have a config already
	if utils.CheckFileExists(CONFIG_LOCATION) {
		b, err := ioutil.ReadFile(CONFIG_LOCATION)
		if err != nil {
			log.Fatalf("Error reading config file: %s", err)
			return err
		}
		if err = json.Unmarshal([]byte(b), &c); err != nil {
			log.Fatalf("Unable to unmarshal config: %s", err)
			return err
		}
		if !c.Validate() {
			log.Fatalf("invalid PulseHA config")
			os.Exit(1)
		}
	} else {
		// Create a default config
		if err := c.SaveDefaultLocalConfig(); err != nil {
			log.Fatalf("unable to load PulseHA config")
			os.Exit(1)
		}
	}
	return nil
}

/**
 * Function used to save the config
 */
func (c *Config) Save() error {
	log.Debug("Config:Save() Saving config..")
	c.Lock()
	defer c.Unlock()
	// Validate before we save
	if !c.Validate() {
		return errors.New("unable to save config. value mismatch")
	}
	// Convert struct back to JSON format
	configJSON, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	// Save back to file
	err = ioutil.WriteFile(CONFIG_LOCATION, configJSON, 0644)
	// Check for errors
	if err != nil {
		log.Error("Unable to save config.json. Either it doesn't exist or there may be a permissions issue")
		return err
	}
	return nil
}

/**
 * Reload the config file into memory.
 * Note: Need to clear memory value before calling Load()
 */
func (c *Config) Reload() {
	log.Info("Reloading PulseHA config")
	if err := c.Load(); err != nil {
		panic(err)
	}
}

/**
 *
 */
func (c *Config) Validate() bool {
	hostname, err := utils.GetHostname()
	if err != nil {
		log.Fatal("Unable to get local hostname")
		return false
	}
	// Make sure our groups section is valid
	if c.Groups == nil {
		log.Fatal("Unable to load Groups section of the config")
		return false
	}

	// Make sure our nodes section is valid
	if c.Nodes == nil {
		log.Fatal("Unable to load Nodes section of the config")
		return false
	}

	// if we are in a cluster.. does our hostname exist?
	if c.ClusterCheck() {
		var exists = func() bool {
			for _, node := range c.Nodes {
				if node.Hostname == hostname {
					return true
				}
			}
			return false
		}
		if !exists() {
			log.Fatal("hostname mismatch. Local hostname does not exist in nodes section")
			return false
		}
	}

	if c.Pulse.FailOverInterval < 1000 || c.Pulse.FailOverLimit < 1000 || c.Pulse.HealthCheckInterval < 1000 {
		log.Fatal("Please make sure the interval and limit values in your config are valid millisecond values of at least 1 second")
		return false
	}

	if c.Pulse.FailOverLimit < c.Pulse.FailOverInterval {
		log.Fatal("The fos_interval value must be a smaller value then your fo_limit")
		return false
	}

	// TODO: Check if we have valid network interface names

	return true
}

// LocalNode - Get the local node object
func (c *Config) LocalNode() Node {
	hostname, err := utils.GetHostname()
	if err != nil {
		return Node{}
	}
	_, node, err :=  c.GetNodeByHostname(hostname)
	if err != nil {
		return Node{}
	}
	return node
}

// ClusterCheck - Check to see if wea re in a configured cluster or not.
func (c *Config) ClusterCheck() bool {
	total := len(c.Nodes)
	if total > 0 {
		// if there is only one node we can assume it's ours
		if total == 1 {
			// make sure we have a bind IP/Port or we are not in a cluster
			hostname, err := utils.GetHostname()
			if err != nil {
				return false
			}
			_, node, err :=  c.GetNodeByHostname(hostname)
			if err != nil {
				return false
			}
			if node.IP == "" && node.Port == "" {
				return false
			}
		}
		return true
	}
	return false
}

/**
Returns the interface the group is assigned to
*/
func (c *Config) GetGroupIface(node string, groupName string) string {
	for nodeName, n := range c.Nodes {
		if nodeName == node {
			for iface, groups := range n.IPGroups {
				for _, group := range groups {
					if group == groupName {
						return iface
					}
				}
			}
		}
	}
	return ""
}

/**
Returns the hostname for a node based of it's IP address
*/
func (c *Config) GetNodeHostnameByAddress(address string) (string, error) {
	for _, node := range c.Nodes {
		if node.IP == address {
			return node.Hostname, nil
		}
	}
	return "", errors.New("unable to find node with IP address " + address)
}

// GetNodeByHostname - Get node by hostname
func (c *Config) GetNodeByHostname(hostname string) (uid string, node Node, err error) {
	for uid, node := range c.Nodes {
		if node.Hostname == hostname  {
			return uid, node, nil
		}
	}
	return "", Node{}, errors.New("unable to find node with hostname " + hostname)
}

// UpdateValue - Update a key's value
func (c *Config) UpdateValue(key string, value string) error {
	if err := jsonHelper.SetStructFieldByTag(key, value, &c.Pulse); err != nil {
		return err
	}
	// Save our config with the updated info
	if err := c.Save(); err != nil {
		return err
	}
	return nil
}

// UpdateHostname - Changes our local node hostname and the hostname in our node section
func (c *Config) UpdateHostname(newHostname string) {
	localNode := c.GetLocalNode()
	// Update our local node hostname
	c.Pulse.LocalNode = newHostname
	// Update the node section hostname
	for _, node := range c.Nodes {
		if node.Hostname == localNode.Hostname {
			node.Hostname = newHostname
		}
	}
}

// DefaultLocalConfig - Generate a default config to write
func (c *Config) SaveDefaultLocalConfig() error {
	defaultConfig := &Config{
		Pulse: Local{
			HealthCheckInterval: 1000,
			FailOverInterval:    5000,
			FailOverLimit:       10000,
			LocalNode:           "",
			ClusterToken:        "",
			LoggingLevel:        "info",
		},
		Groups: map[string][]string{},
		Nodes:  map[string]Node{},
	}
	// Convert struct back to JSON format
	configJSON, err := json.MarshalIndent(defaultConfig, "", "    ")
	if err != nil {
		return err
	}
	// Set our config in memory
	c.Pulse = defaultConfig.Pulse
	c.Groups = defaultConfig.Groups
	c.Nodes = defaultConfig.Nodes
	// Save back to file
	err = ioutil.WriteFile(CONFIG_LOCATION, configJSON, 0644)
	// Check for errors
	if err != nil {
		log.Error("Unable to save config.json. There may be a permissions issue")
		return err
	}
	return nil
}
